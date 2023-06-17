package test

import (
	"flag"
	"fmt"
	"log"
	"testing"

	"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/kernel"
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

var start bool
var noShutdown bool
var tag string
var Overlays bool

func init() {
	flag.StringVar(&tag, "tag", "", "Docker image tag")
	flag.BoolVar(&start, "start", false, "Start system")
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
		ts, err := makeSysClnt(t, BOOT_ALL)
		if err != nil {
			return nil, err
		}
		//if err := ts.switchNamed(); err != nil {
		//	ts.Shutdown()
		//return nil, err
		//}
		return ts, nil
	}
}

// Replace knamed with a proc-based named
func (ts *Tstate) switchNamed() error {
	ts.proc = proc.MakeProc("named", []string{sp.ROOTREALM.String(), "0"})
	if err := ts.Spawn(ts.proc); err != nil {
		log.Printf("spawn failure %v\n", err)
		return err
	}
	log.Printf("wait for start named\n")
	if err := ts.WaitStart(ts.proc.GetPid()); err != nil {
		return err
	}
	log.Printf("named started\n")
	if err := ts.StopKNamed(); err != nil {
		log.Printf("killed named err %v\n", err)
	}
	log.Printf("killed named\n")
	return nil
}

func makeSysClnt(t *testing.T, srvs string) (*Tstate, error) {
	namedport := sp.MkTaddrs([]string{NAMEDPORT})
	kernelid := ""
	var containerIP string
	var err error
	var k *bootkernelclnt.Kernel
	if start {
		kernelid = bootkernelclnt.GenKernelId()
		ip, err := bootkernelclnt.Start(kernelid, tag, srvs, namedport, Overlays)
		if err != nil {
			return nil, err
		}
		containerIP = ip
	}
	proc.SetPid(proc.Tpid("test-" + proc.GenPid().String()))
	namedAddr, err := kernel.SetNamedIP(containerIP, namedport)
	if err != nil {
		return nil, err
	}
	k, err = bootkernelclnt.MkKernelClnt(kernelid, "test", containerIP, namedAddr)
	if err != nil {
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
		kclnt, err := bootkernelclnt.MkKernelClntStart(tag, "kclnt", BOOT_NODE, ts.NamedAddr(), Overlays)
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

func (ts *Tstate) StopKNamed() error {
	return ts.kclnts[0].Kill(sp.NAMEDREL)
}

func (ts *Tstate) KillOne(s string) error {
	idx := ts.killidx
	ts.killidx++
	return ts.kclnts[idx].Kill(s)
}

func (ts *Tstate) MakeClnt(idx int, name string) (*sigmaclnt.SigmaClnt, error) {
	return ts.kclnts[idx].MkSigmaClnt(name)
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
				return err
			}
		}
	}
	return nil
}
