package test

import (
	"flag"
	"fmt"
	"testing"

	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	NAMEDPORT = ":1111"
)

var containerIP string

func init() {
	flag.StringVar(&containerIP, "containerIP", "127.0.0.1", "IP addr for container")
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
	kclnt *kernelclnt.KernelClnt
	T     *testing.T
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	b, err := makeClntPath(t, path)
	if err != nil {
		db.DFatalf("MakeTstatePath: %v\n", err)
	}
	return b
}

func MakeTstate(t *testing.T) *Tstate {
	ts, err := makeClnt(t)
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return ts
}

func MakeTstateAll(t *testing.T) *Tstate {
	ts, err := makeClnt(t)
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return ts
}

func makeClntPath(t *testing.T, path string) (*Tstate, error) {
	if path == sp.NAMED {
		return makeClnt(t)
	} else {
		ts, err := makeClnt(t)
		if err != nil {
			return nil, err
		}
		ts.RmDir(path)
		ts.MkDir(path, 0777)
		return ts, nil
	}
}

// Join a realm/set of machines are already running  XXX to compile
func JoinRealm(t *testing.T, realmid string) (*Tstate, error) {
	//fsl, pclnt, err := mkClient("", realmid, []string{""}) // XXX get it from rconfig
	//if err != nil {
	//	return nil, err
	//}
	//rconfig := realm.GetRealmConfig(fsl, realmid)
	db.DFatalf("Unimplemented")
	return nil, nil
}

func makeClnt(t *testing.T) (*Tstate, error) {
	proc.SetPid(proc.Tpid("test-" + proc.GenPid().String()))
	namedAddr, err := kernel.SetNamedIP(containerIP, []string{NAMEDPORT})
	if err != nil {
		return nil, err
	}
	sc, err := sigmaclnt.MkSigmaClntProc("test", containerIP, namedAddr)
	if err != nil {
		return nil, err
	}
	kclnt, err := kernelclnt.MakeKernelClnt(sc.FsLib, sp.BOOT+"~local/")
	if err != nil {
		return nil, err
	}
	return &Tstate{sc, kclnt, t}, nil
}

func (ts *Tstate) BootNode(n int) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (ts *Tstate) Boot(s string) error {
	pid, err := ts.kclnt.Boot(s, sp.Taddrs{})
	if err != nil {
		return err
	}
	db.DFatalf("Unimplemented %v", pid)
	return nil
}

func (ts *Tstate) KillOne(s string) error {
	return ts.kclnt.Kill(s)
}

func (ts *Tstate) Shutdown() error {
	db.DPrintf(db.TEST, "Shutdown")
	return ts.kclnt.Shutdown()
}
