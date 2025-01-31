// Package sets up an environment for testing sigmaos.  If running
// test with --start, test will start sigmaos kernel.  Without
// --start, it will test create a kernelclnt without starting kernel.
package shellctx

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	bootclnt "sigmaos/boot/clnt"
	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	realmpkg "sigmaos/realm"
	realmclnt "sigmaos/realm/clnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/auth"
	iputil "sigmaos/util/ip"
)

var Start bool = true
var reuseKernel bool = false
var noShutdown bool = false
var tag string = sp.LOCAL_BUILD
var EtcdIP string = "127.0.0.1"
var useSPProxy bool = false
var noDialProxy bool = false
var noBootDialProxy bool = false
var Withs3pathclnt bool = false

// flag.StringVar(&sp.Version, "version", sp.DEFAULT_VERSION, "Build version")

var savedTstate *Tstate

type Tstate struct {
	ntype bootclnt.Tboot
	*sigmaclnt.SigmaClnt
	rc      *realmclnt.RealmClnt
	memfs   *proc.Proc
	Kclnts  []*bootclnt.Kernel
	killidx int
	T       *testing.T
	proc    *proc.Proc
	spkc    *bootclnt.Kernel
}

func NewTstatePath(path string) (*Tstate, error) {
	ts, err := newSysClntPath(path)
	if err != nil {
		db.DPrintf(db.ERROR, "NewTstatePath: %v\n", err)
		return nil, err
	}
	if Withs3pathclnt {
		err := ts.MountS3PathClnt()
		if err != nil {
			fmt.Printf("Error MountS3PathClnt: %v", err)
			return nil, err
		}
	}
	if path == filepath.Join(sp.MEMFS, sp.LOCAL)+"/" {
		ts.memfs = proc.NewProc("memfsd", []string{})
		err := ts.Spawn(ts.memfs)
		if err != nil {
			fmt.Printf("Error Spawn memfsd: %v", err)
			return nil, err
		}
		err = ts.WaitStart(ts.memfs.GetPid())
		if err != nil {
			fmt.Printf("Error WaitStart memfsd: %v", err)
			return nil, err
		}
	}
	return ts, nil
}

func NewTstate() (*Tstate, error) {
	return NewTstatePath(sp.NAMED)
}

func NewTstateAll() (*Tstate, error) {
	return NewTstatePath("all")
}

func NewTstateWithRealms() (*Tstate, error) {
	ts, err := newSysClnt(bootclnt.BOOT_REALM)
	if err != nil {
		db.DPrintf(db.ERROR, "NewTstateRealm: %v\n", err)
		return nil, err
	}
	rc, err := realmclnt.NewRealmClnt(ts.FsLib)
	if err != nil {
		db.DPrintf(db.ERROR, "NewRealmClnt make realmclnt: %v\n", err)
		return nil, err
	}
	ts.rc = rc
	return ts, nil
}

func newSysClntPath(path string) (*Tstate, error) {
	if path == sp.NAMED {
		return newSysClnt(bootclnt.BOOT_NAMED)
	} else {
		return newSysClnt(bootclnt.BOOT_ALL)
	}
}

func newSysClnt(ntype bootclnt.Tboot) (*Tstate, error) {
	// If the tests are invoked trying to reuse booted systems, and the same
	// servers are meant to be booted, skip the boot.
	if reuseKernel && savedTstate != nil && savedTstate.ntype == ntype {
		// Reset the Tstate's *testing.T
		savedTstate.T = nil
		db.DPrintf(db.TEST, "Reusing previously-booted system")
		return savedTstate, nil
	}
	localIP, err1 := iputil.LocalIP()
	if err1 != nil {
		db.DPrintf(db.ERROR, "Error local IP: %v", err1)
		return nil, err1
	}
	s3secrets, err1 := auth.GetAWSSecrets(sp.AWS_PROFILE)
	if err1 != nil {
		db.DPrintf(db.ERROR, "Failed to load AWS secrets %v", err1)
		return nil, err1
	}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(sp.Tip(EtcdIP))
	if err != nil {
		fmt.Printf("Error NewFsEtcdEndpoint: %v", err)
		return nil, err
	}
	secrets := map[string]*sp.SecretProto{"s3": s3secrets}
	useDialProxy := !noDialProxy
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, etcdMnt, localIP, localIP, tag, useSPProxy, useDialProxy)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	var kernelid string
	var k *bootclnt.Kernel
	if Start {
		kernelid = bootclnt.GenKernelId()
		_, err := bootclnt.Start(kernelid, sp.Tip(EtcdIP), pe, ntype, useDialProxy)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start kernel")
			return nil, err
		}
	}
	var spkc *bootclnt.Kernel
	if !noBootDialProxy && (useSPProxy || useDialProxy) {
		db.DPrintf(db.BOOT, "Booting spproxyd: usespproxyd %v usedialproxy %v", useSPProxy, useDialProxy)
		sckid := sp.SPProxydKernel(bootclnt.GenKernelId())
		_, err := bootclnt.Start(sckid, sp.Tip(EtcdIP), pe, sp.SPPROXYDREL, useDialProxy)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start kernel for spproxyd")
			return nil, err
		}
		spkc, err = bootclnt.NewKernelClnt(sckid, sp.Tip(EtcdIP), pe)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error make kernel clnt for spproxyd")
			return nil, err
		}
	}
	k, err = bootclnt.NewKernelClnt(kernelid, sp.Tip(EtcdIP), pe)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error make kernel clnt")
		return nil, err
	}
	savedTstate = &Tstate{
		ntype:     ntype,
		SigmaClnt: k.SigmaClnt,
		Kclnts:    []*bootclnt.Kernel{k},
		killidx:   0,
		T:         nil,
		spkc:      spkc,
	}
	return savedTstate, nil
}

func (ts *Tstate) Shutdown() error {
	// If the test asked for a lease at some point, clear the saved Tstate to
	// avoid having leases carry over to the next test
	if ts.AskedForLease() || ts.Disconnected() {
		savedTstate = nil
	}
	// If the developer chose to reuse the kernel, and there is a saved kernel
	// (meaning that the saved kernel hasn't been cleared, e.g., due to a crash
	// test), then skip shutdown
	if reuseKernel && savedTstate != nil {
		db.DPrintf(db.ALWAYS, "Skipping shutdown to reuse kernel")
		db.DPrintf(db.TEST, "Skipping shutdown to reuse kernel")
		return nil
	}

	db.DPrintf(db.TEST, "Shutdown")
	defer db.DPrintf(db.TEST, "Done Shutdown")

	if noShutdown {
		db.DPrintf(db.ALWAYS, "Skipping shutdown")
		db.DPrintf(db.TEST, "Skipping shutdown")
	} else {
		db.DPrintf(db.SYSTEM, "Shutdown")
		if ts.memfs != nil {
			db.DPrintf(db.SYSTEM, "Shutdown memfs")
			err := ts.Evict(ts.memfs.GetPid())
			assert.Nil(ts.T, err, "evict")
			_, err = ts.WaitExit(ts.memfs.GetPid())
			assert.Nil(ts.T, err, "WaitExit error")
		}
		if err := ts.RmDir(ts.ProcEnv().ProcDir); err != nil {
			db.DPrintf(db.ALWAYS, "Failed to clean up %v err %v", ts.ProcEnv().ProcDir, err)
		}
		// Shut down kernels; the one running named last
		for i := len(ts.Kclnts) - 1; i >= 0; i-- {
			if err := ts.Kclnts[i].Shutdown(); err != nil {
				db.DPrintf(db.ALWAYS, "Shutdown %v err %v", ts.Kclnts[i].KernelId(), err)
			}
			ts.Kclnts[i].Close()
		}
		if ts.spkc != nil {
			if err := ts.spkc.Shutdown(); err != nil {
				db.DPrintf(db.ALWAYS, "Shutdown spproxyd err %v", err)
			}
			ts.spkc.Close()
		}
	}
	return nil
}

// Tstate relative to a realm.
type RealmTstate struct {
	realm sp.Trealm
	*sigmaclnt.SigmaClnt
	Ts *Tstate
}

// Creates a realm, and a tstate relative to that realm.
func NewRealmTstate(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, true, realmpkg.SUBSYSTEM_PER_NODE, realmpkg.SUBSYSTEM_PER_NODE)
}

func NewRealmTstateNumSubsystems(ts *Tstate, realm sp.Trealm, numS3 int64, numUX int64) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, true, numS3, numUX)
}

// Creates a tstate relative to an existing realm.
func NewRealmTstateClnt(ts *Tstate, realm sp.Trealm) (*RealmTstate, error) {
	return newRealmTstateClnt(ts, realm, false, realmpkg.SUBSYSTEM_PER_NODE, realmpkg.SUBSYSTEM_PER_NODE)
}

func newRealmTstateClnt(ts *Tstate, realm sp.Trealm, newrealm bool, numS3 int64, numUX int64) (*RealmTstate, error) {
	if newrealm {
		net := ""
		db.DPrintf(db.TEST, "Make realm %v", realm)
		if err := ts.rc.NewRealm(realm, net, numS3, numUX); err != nil {
			db.DPrintf(db.ERROR, "Error NewRealmTstate NewRealm: %v", err)
			return nil, err
		}
		db.DPrintf(db.TEST, "Done making realm %v", realm)
	}
	pe := proc.NewDifferentRealmProcEnv(ts.ProcEnv(), realm)
	db.DPrintf(db.TEST, "ProcEnv for new realm %v", pe)
	if sc, err := sigmaclnt.NewSigmaClntRootInit(pe); err != nil {
		db.DPrintf(db.ERROR, "Error NewRealmTstate NewSigmaClnt: %v", err)
		return nil, err
	} else {
		return &RealmTstate{
			realm:     realm,
			SigmaClnt: sc,
			Ts:        ts,
		}, nil
	}
}

func (rts *RealmTstate) GetRealm() sp.Trealm {
	return rts.realm
}

func (rts *RealmTstate) Remove() error {
	return rts.Ts.rc.RemoveRealm(rts.realm)
}
