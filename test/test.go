package test

import (
	"flag"
	"fmt"
	"testing"

	"sigmaos/bootkernelclnt"
	"sigmaos/config"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/realmclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// If running test with --start, test will start sigmaos kernel.
// Without --start, it will test create a kernelclnt without starting
// kernel.
//

const (
	BOOT_REALM = "realm"
	BOOT_ALL   = "all"
	BOOT_NAMED = "named"
	BOOT_NODE  = "node"

	NAMEDPORT = ":1111"
)

var Start bool
var noShutdown bool
var tag string
var etcdIP string
var Overlays bool

func init() {
	flag.StringVar(&etcdIP, "etcdIP", "127.0.0.1", "Etcd IP")
	flag.StringVar(&tag, "tag", "", "Docker image tag")
	flag.BoolVar(&Start, "start", false, "Start system")
	flag.BoolVar(&noShutdown, "no-shutdown", false, "Don't shut down the system")
	flag.BoolVar(&Overlays, "overlays", false, "Overlays")
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
	kclnts  []*bootkernelclnt.Kernel
	killidx int
	T       *testing.T
	proc    *proc.Proc
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	ts, err := makeSysClntPath(t, path)
	if err != nil {
		db.DFatalf("MakeTstatePath: %v\n", err)
	}
	return ts
}

func MakeTstate(t *testing.T) *Tstate {
	return MakeTstatePath(t, sp.NAMED)
}

func MakeTstateAll(t *testing.T) *Tstate {
	return MakeTstatePath(t, "all")
}

func MakeTstateWithRealms(t *testing.T) *Tstate {
	ts, err := makeSysClnt(t, BOOT_REALM)
	if err != nil {
		db.DFatalf("MakeTstateRealm: %v\n", err)
	}
	rc, err := realmclnt.MakeRealmClnt(ts.FsLib)
	if err != nil {
		db.DFatalf("MakeRealmClnt make realmclnt: %v\n", err)
	}
	ts.rc = rc
	return ts
}

func makeSysClntPath(t *testing.T, path string) (*Tstate, error) {
	if path == sp.NAMED {
		return makeSysClnt(t, BOOT_NAMED)
	} else {
		return makeSysClnt(t, BOOT_ALL)
	}
}

func makeSysClnt(t *testing.T, srvs string) (*Tstate, error) {
	// XXX What should we set localIP to?
	localIP, err1 := container.LocalIP()
	if err1 != nil {
		db.DFatalf("Error local IP: %v", err1)
	}
	scfg := config.NewTestSigmaConfig(sp.ROOTREALM, etcdIP, localIP, tag)
	var kernelid string
	var err error
	var k *bootkernelclnt.Kernel
	if Start {
		kernelid = bootkernelclnt.GenKernelId()
		ip, err := bootkernelclnt.Start(kernelid, scfg, srvs, Overlays)
		db.DPrintf(db.ALWAYS, "Got ip %v", ip)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error start kernel")
			return nil, err
		}
	}
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error set named ip")
		return nil, err
	}
	k, err = bootkernelclnt.MkKernelClnt(kernelid, scfg)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error make kernel clnt")
		return nil, err
	}
	return &Tstate{
		SigmaClnt: k.SigmaClnt,
		kclnts:    []*bootkernelclnt.Kernel{k},
		killidx:   0,
		T:         t,
	}, nil
}

func (ts *Tstate) BootNode(n int) error {
	for i := 0; i < n; i++ {
		kclnt, err := bootkernelclnt.MkKernelClntStart(ts.SigmaConfig(), BOOT_NODE, Overlays)
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

func (ts *Tstate) MakeClnt(idx int, scfg *config.SigmaConfig) (*sigmaclnt.SigmaClnt, error) {
	db.DFatalf("Error: pass sigma config")
	return ts.kclnts[idx].NewSigmaClnt(scfg)
}

func (ts *Tstate) Shutdown() error {
	db.DPrintf(db.TEST, "Shutdown")
	defer db.DPrintf(db.TEST, "Done Shutdown")
	if noShutdown {
		db.DPrintf(db.ALWAYS, "Skipping shutdown")
		db.DPrintf(db.TEST, "Skipping shutdown")
	} else {
		db.DPrintf(db.SYSTEM, "Shutdown")
		if err := ts.RmDir(proc.GetProcDir()); err != nil {
			db.DPrintf(db.ALWAYS, "Failed to clean up %v err %v", proc.GetProcDir(), err)
		}
		// Shut down other kernel; the one running named last
		for i := len(ts.kclnts) - 1; i >= 0; i-- {
			if err := ts.kclnts[i].Shutdown(); err != nil {
				db.DPrintf(db.ALWAYS, "Shutdown %v err %v", ts.kclnts[i].KernelId, err)
			}
		}
	}
	return nil
}
