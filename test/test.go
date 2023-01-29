package test

import (
	"flag"
	"fmt"
	"testing"

	"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	BOOT_REALM = "named;realmd;schedd;ux;s3;db"
	BOOT_ALL   = "named;schedd;ux;s3;db"
	BOOT_NAMED = "named"
	BOOT_NODE  = "schedd;ux;s3;db"

	NAMEDPORT = ":1111"
)

var containerIP string
var start bool

func init() {
	flag.StringVar(&containerIP, "containerIP", "127.0.0.1", "IP addr for container")
	flag.BoolVar(&start, "start", false, "Start system")
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
	kclnts []*bootkernelclnt.Kernel
	T      *testing.T
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	b, err := makeSysClntPath(t, path)
	if err != nil {
		db.DFatalf("MakeTstatePath: %v\n", err)
	}
	return b
}

func MakeTstate(t *testing.T) *Tstate {
	ts, err := makeSysClnt(t, BOOT_NAMED)
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return ts
}

func MakeTstateAll(t *testing.T) *Tstate {
	ts, err := makeSysClnt(t, BOOT_ALL)
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return ts
}

func MakeTstateRealm(t *testing.T) *Tstate {
	ts, err := makeSysClnt(t, BOOT_REALM)
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
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
		ts.RmDir(path)
		ts.MkDir(path, 0777)
		return ts, nil
	}
}

func makeSysClnt(t *testing.T, srvs string) (*Tstate, error) {
	namedport := []string{NAMEDPORT}
	if start {
		ip, err := bootkernelclnt.Start(srvs, namedport)
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
	k, err := bootkernelclnt.MkKernelClnt("test", containerIP, namedAddr)
	if err != nil {
		return nil, err
	}
	return &Tstate{k.SigmaClnt, []*bootkernelclnt.Kernel{k}, t}, nil
}

func (ts *Tstate) BootNode(n int) error {
	for i := 0; i < n; i++ {
		kclnt, err := bootkernelclnt.MkBootKernelClnt("kclnt", BOOT_NODE, ts.NamedAddr())
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
	return ts.kclnts[0].Kill(s)
}

func (ts *Tstate) Shutdown() error {
	db.DPrintf(db.SYSTEM, "Shutdown")
	for i := len(ts.kclnts) - 1; i >= 0; i-- {
		db.DPrintf(db.SYSTEM, "Shutdown kernel %v", i)
		// XXX shut down other kernels first?
		if err := ts.kclnts[i].Shutdown(); err != nil {
			return err
		}
		db.DPrintf(db.SYSTEM, "Done shutdown kernel %v", i)
	}
	return nil
}
