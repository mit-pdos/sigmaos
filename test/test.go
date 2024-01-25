// Package sets up an environment for testing sigmaos.  If running
// test with --start, test will start sigmaos kernel.  Without
// --start, it will test create a kernelclnt without starting kernel.
package test

import (
	"flag"
	"fmt"
	gopath "path"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/auth"
	"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/netsigma"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/realmclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	BOOT_REALM = "realm"
	BOOT_ALL   = "all"
	BOOT_NAMED = "named"
	BOOT_NODE  = "node"
)

var Start bool
var noShutdown bool
var tag string
var EtcdIP string
var Overlays bool
var GVisor bool
var useSigmaclntd bool

func init() {
	flag.StringVar(&EtcdIP, "etcdIP", "127.0.0.1", "Etcd IP")
	flag.StringVar(&tag, "tag", sp.LOCAL_BUILD, "Docker image tag")
	flag.BoolVar(&Start, "start", false, "Start system")
	flag.BoolVar(&noShutdown, "no-shutdown", false, "Don't shut down the system")
	flag.BoolVar(&Overlays, "overlays", false, "Overlays")
	flag.BoolVar(&GVisor, "gvisor", false, "GVisor")
	flag.BoolVar(&useSigmaclntd, "usesigmaclntd", false, "Use sigmaclntd?")
}

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

type Tstate struct {
	*sigmaclnt.SigmaClnt
	rc      *realmclnt.RealmClnt
	memfs   *proc.Proc
	kclnts  []*bootkernelclnt.Kernel
	killidx int
	T       *testing.T
	proc    *proc.Proc
	scsck   *bootkernelclnt.Kernel
	as      auth.AuthSrv
}

func NewTstatePath(t *testing.T, path string) (*Tstate, error) {
	ts, err := newSysClntPath(t, path)
	if err != nil {
		db.DPrintf(db.ERROR, "NewTstatePath: %v\n", err)
		return nil, err
	}
	if path == gopath.Join(sp.MEMFS, "~local/")+"/" {
		ts.memfs = proc.NewProc("memfsd", []string{})
		err := ts.Spawn(ts.memfs)
		assert.Nil(t, err)
		err = ts.WaitStart(ts.memfs.GetPid())
		assert.Nil(t, err, "WaitStart error")
	}
	return ts, nil
}

func NewTstate(t *testing.T) (*Tstate, error) {
	return NewTstatePath(t, sp.NAMED)
}

func NewTstateAll(t *testing.T) (*Tstate, error) {
	return NewTstatePath(t, "all")
}

func NewTstateWithRealms(t *testing.T) (*Tstate, error) {
	ts, err := newSysClnt(t, BOOT_REALM)
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

func newSysClntPath(t *testing.T, path string) (*Tstate, error) {
	if path == sp.NAMED {
		return newSysClnt(t, BOOT_NAMED)
	} else {
		return newSysClnt(t, BOOT_ALL)
	}
}

func newSysClnt(t *testing.T, srvs string) (*Tstate, error) {
	localIP, err1 := netsigma.LocalIP()
	if err1 != nil {
		db.DPrintf(db.ERROR, "Error local IP: %v", err1)
		return nil, err1
	}
	as, err1 := auth.NewHMACAuthSrv([]byte("PDOS"))
	if err1 != nil {
		db.DPrintf(db.ERROR, "Error NewAuthSrv: %v", err1)
		return nil, err1
	}
	pe := proc.NewTestProcEnv(sp.ROOTREALM, sp.Tip(EtcdIP), localIP, localIP, tag, Overlays, useSigmaclntd)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	pc := auth.NewProcClaims(pe)
	token, err1 := as.NewToken(pc)
	if err1 != nil {
		db.DPrintf(db.ERROR, "Error NewToken: %v", err1)
		return nil, err1
	}
	pe.SetToken(token)
	var kernelid string
	var err error
	var k *bootkernelclnt.Kernel
	if Start {
		kernelid = bootkernelclnt.GenKernelId()
		_, err := bootkernelclnt.Start(kernelid, pe, srvs, Overlays, GVisor)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start kernel")
			return nil, err
		}
	}
	var scsck *bootkernelclnt.Kernel
	var sckid string
	if useSigmaclntd {
		db.DPrintf(db.BOOT, "Use sigmaclntd")
		sckid = bootkernelclnt.GenKernelId()
		_, err := bootkernelclnt.Start(sckid, pe, sp.SIGMACLNTDREL, Overlays, GVisor)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start kernel for sigmaclntd")
			return nil, err
		}
		scsck, err = bootkernelclnt.NewKernelClnt(sckid, pe)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error make kernel clnt for sigmaclntd")
			return nil, err
		}
	}
	k, err = bootkernelclnt.NewKernelClnt(kernelid, pe)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error make kernel clnt")
		return nil, err
	}
	return &Tstate{
		SigmaClnt: k.SigmaClnt,
		kclnts:    []*bootkernelclnt.Kernel{k},
		killidx:   0,
		T:         t,
		scsck:     scsck,
		as:        as,
	}, nil
}

func (ts *Tstate) BootNode(n int) error {
	for i := 0; i < n; i++ {
		kclnt, err := bootkernelclnt.NewKernelClntStart(ts.ProcEnv(), BOOT_NODE, Overlays, GVisor)
		if err != nil {
			return err
		}
		ts.kclnts = append(ts.kclnts, kclnt)
	}
	return nil
}

func (ts *Tstate) Boot(s string) error {
	return ts.kclnts[0].Boot(s)
}

func (ts *Tstate) BootFss3d() error {
	return ts.Boot(sp.S3REL)
}

func (ts *Tstate) KillOne(s string) error {
	idx := ts.killidx
	ts.killidx++
	return ts.kclnts[idx].Kill(s)
}

func (ts *Tstate) NewClnt(idx int, pe *proc.ProcEnv) (*sigmaclnt.SigmaClnt, error) {
	return ts.kclnts[idx].NewSigmaClnt(pe)
}

func (ts *Tstate) Shutdown() error {
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
				db.DPrintf(db.ALWAYS, "Shutdown %v err %v", ts.kclnts[i].KernelId, err)
			}
			// ts.kclnts[i].Close()
		}
		if ts.scsck != nil {
			if err := ts.scsck.Shutdown(); err != nil {
				db.DPrintf(db.ALWAYS, "Shutdown sigmaclntd err %v", err)
			}
		}
	}
	return nil
}

func Dump(t *testing.T) {
	pe := proc.NewTestProcEnv(sp.ROOTREALM, sp.Tip(EtcdIP), "", "", "", false, false)
	fs, err := fsetcd.NewFsEtcd(pe.GetRealm(), pe.GetEtcdIP())
	assert.Nil(t, err)
	nd, err := fs.ReadDir(fsetcd.ROOT)
	assert.Nil(t, err)
	err = fs.Dump(0, nd, path.Path{}, fsetcd.ROOT)
	assert.Nil(t, err)
}
