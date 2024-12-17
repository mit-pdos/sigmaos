// Package sets up an environment for testing sigmaos.  If running
// test with --start, test will start sigmaos kernel.  Without
// --start, it will test create a kernelclnt without starting kernel.
package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/auth"
	"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/realmclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	BOOT_REALM   = "realm"
	BOOT_ALL     = "all"
	BOOT_NAMED   = "named"
	BOOT_NODE    = "node"
	BOOT_MINNODE = "minnode"
)

var Start bool = true
var reuseKernel bool = false
var noShutdown bool = false
var tag string = sp.LOCAL_BUILD
var EtcdIP string = "127.0.0.1"
var Overlays bool = false
var GVisor bool = false
var useSPProxy bool = false
var noNetProxy bool = false
var noBootNetProxy bool = false
var Withs3pathclnt bool = false
var Version string = sp.DEFAULT_VERSION
var savedTstate *Tstate

func Mbyte(sz sp.Tlength) float64 {
	return float64(sz) / float64(sp.MBYTE)
}

func TputStr(sz sp.Tlength, ms int64) string {
	s := float64(ms) / 1000
	return fmt.Sprintf("%.2fMB/s", Mbyte(sz)/s)
}

func Tput(sz sp.Tlength, ms int64) float64 {
	t := float64(ms) / 1000
	return Mbyte(sz) / t
}

type TstateMin struct {
	T    *testing.T
	lip  sp.Tip
	PE   *proc.ProcEnv
	Addr *sp.Taddr
}

func NewTstateMinAddr(addr *sp.Taddr) *TstateMin {
	s3secrets, err1 := auth.GetAWSSecrets(sp.AWS_PROFILE)
	if err1 != nil {
		fmt.Printf("Failed to load AWS secrets %v", err1)
		return nil
	}
	secrets := map[string]*sp.SecretProto{"s3": s3secrets}
	lip := sp.Tip("127.0.0.1")
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(sp.Tip(EtcdIP))
	if err != nil {
		fmt.Printf("Error NewFsEtcdEndpoint: %v", err)
		return nil
	}
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, etcdMnt, lip, lip, "", false, false, false)
	pe.Program = "srv"
	pe.SetPrincipal(sp.NewPrincipal("srv", sp.ROOTREALM))
	proc.SetSigmaDebugPid(pe.GetPID().String())
	return &TstateMin{
		T:    nil,
		lip:  lip,
		PE:   pe,
		Addr: addr,
	}
}

func NewTstateMin() *TstateMin {
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	return NewTstateMinAddr(addr)
}

type Tstate struct {
	srvs string
	*sigmaclnt.SigmaClnt
	rc            *realmclnt.RealmClnt
	memfs         *proc.Proc
	kclnts        []*bootkernelclnt.Kernel
	killidx       int
	T             *testing.T
	proc          *proc.Proc
	scsck         *bootkernelclnt.Kernel
	masterPubKey  auth.PublicKey
	masterPrivKey auth.PrivateKey
}

func NewTstatePath(path string) (*Tstate, error) {
	if Overlays && EtcdIP == "127.0.0.1" {
		return nil, fmt.Errorf("Overlays set, but etcdIP not set")
	}
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
	if path == filepath.Join(sp.MEMFS, "~local/")+"/" {
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

func NewTstate(t *testing.T) (*Tstate, error) {
	return NewTstatePath(sp.NAMED)
}

func NewTstateAll(t *testing.T) (*Tstate, error) {
	return NewTstatePath("all")
}

func NewTstateWithRealms() (*Tstate, error) {
	ts, err := newSysClnt(BOOT_REALM)
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
		return newSysClnt(BOOT_NAMED)
	} else {
		return newSysClnt(BOOT_ALL)
	}
}

func newSysClnt(srvs string) (*Tstate, error) {
	// If the tests are invoked trying to reuse booted systems, and the same
	// servers are meant to be booted, skip the boot.
	if reuseKernel && savedTstate != nil && savedTstate.srvs == srvs {
		// Reset the Tstate's *testing.T
		savedTstate.T = nil
		db.DPrintf(db.TEST, "Reusing previously-booted system")
		return savedTstate, nil
	}
	localIP, err1 := netsigma.LocalIP()
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
	useNetProxy := !noNetProxy
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, etcdMnt, localIP, localIP, tag, Overlays, useSPProxy, useNetProxy)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	var kernelid string
	var k *bootkernelclnt.Kernel
	if Start {
		kernelid = bootkernelclnt.GenKernelId()
		fmt.Printf("kernelid: %v\n", kernelid)
		_, err := bootkernelclnt.Start(kernelid, sp.Tip(EtcdIP), pe, srvs, Overlays, GVisor, useNetProxy)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start kernel")
			return nil, err
		}
	}
	var scsck *bootkernelclnt.Kernel
	var sckid string
	if !noBootNetProxy && (useSPProxy || useNetProxy) {
		db.DPrintf(db.BOOT, "Booting spproxyd: usespproxyd %v usenetproxy %v", useSPProxy, useNetProxy)
		sckid = sp.SPProxydKernel(bootkernelclnt.GenKernelId())
		_, err := bootkernelclnt.Start(sckid, sp.Tip(EtcdIP), pe, sp.SPPROXYDREL, Overlays, GVisor, useNetProxy)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start kernel for spproxyd")
			return nil, err
		}
		scsck, err = bootkernelclnt.NewKernelClnt(sckid, sp.Tip(EtcdIP), pe)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error make kernel clnt for spproxyd")
			return nil, err
		}
	}
	k, err = bootkernelclnt.NewKernelClnt(kernelid, sp.Tip(EtcdIP), pe)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error make kernel clnt")
		return nil, err
	}
	savedTstate = &Tstate{
		srvs:      srvs,
		SigmaClnt: k.SigmaClnt,
		kclnts:    []*bootkernelclnt.Kernel{k},
		killidx:   0,
		T:         nil,
		scsck:     scsck,
	}
	return savedTstate, nil
}

func (ts *Tstate) BootMinNode(n int) error {
	return ts.bootNode(n, BOOT_MINNODE)
}

func (ts *Tstate) BootNode(n int) error {
	return ts.bootNode(n, BOOT_NODE)
}

func (ts *Tstate) bootNode(n int, nodetype string) error {
	useNetProxy := !noNetProxy
	// Clear the saved kernel, since the next test may not need an additional
	// node
	savedTstate = nil
	for i := 0; i < n; i++ {
		kclnt, err := bootkernelclnt.NewKernelClntStart(sp.Tip(EtcdIP), ts.ProcEnv(), nodetype, Overlays, GVisor, useNetProxy)
		if err != nil {
			return err
		}
		ts.kclnts = append(ts.kclnts, kclnt)
	}
	return nil
}

func (ts *Tstate) GetKernelClnt(n int) *bootkernelclnt.Kernel {
	return ts.kclnts[n]
}

func (ts *Tstate) Boot(s string) error {
	// Clear the saved kernel, since the next test may not need an additional
	// node
	savedTstate = nil
	return ts.kclnts[0].Boot(s)
}

func (ts *Tstate) BootFss3d() error {
	// Clear the saved kernel, since the next test may not need an additional
	// node
	savedTstate = nil
	return ts.Boot(sp.S3REL)
}

func (ts *Tstate) KillOne(s string) error {
	idx := ts.killidx
	ts.killidx++
	// Clear the saved kernel, to make sure it is fully shut down and then
	// brought back up again before the next test
	savedTstate = nil
	return ts.kclnts[idx].Kill(s)
}

func (ts *Tstate) NewClnt(idx int, pe *proc.ProcEnv) (*sigmaclnt.SigmaClnt, error) {
	return ts.kclnts[idx].NewSigmaClnt(pe)
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
		for i := len(ts.kclnts) - 1; i >= 0; i-- {
			if err := ts.kclnts[i].Shutdown(); err != nil {
				db.DPrintf(db.ALWAYS, "Shutdown %v err %v", ts.kclnts[i].KernelId(), err)
			}
			ts.kclnts[i].Close()
		}
		if ts.scsck != nil {
			if err := ts.scsck.Shutdown(); err != nil {
				db.DPrintf(db.ALWAYS, "Shutdown spproxyd err %v", err)
			}
			ts.scsck.Close()
		}
	}
	return nil
}
