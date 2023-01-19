package test

import (
	"flag"
	"fmt"
	"testing"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/system"
)

const (
	ROOTREALM = "rootrealm"
)

var realmid string // Use this realm to run tests instead of starting a new one. This is used for multi-machine tests.

// Read & set the proc version.
func init() {
	flag.StringVar(&realmid, "realm", ROOTREALM, "realm id")
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
	*system.System
	*fslib.FsLib
	*procclnt.ProcClnt
	T *testing.T
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	b, err := bootPath(t, path)
	if err != nil {
		db.DFatalf("MakeTstatePath: %v\n", err)
	}
	return b
}

func MakeTstate(t *testing.T) *Tstate {
	ts, err := bootSystem(t, false)
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return ts
}

func MakeTstateAll(t *testing.T) *Tstate {
	ts, err := bootSystem(t, true)
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return ts
}

func bootPath(t *testing.T, path string) (*Tstate, error) {
	if path == sp.NAMED {
		return bootSystem(t, false)
	} else {
		ts, err := bootSystem(t, true)
		if err != nil {
			return nil, err
		}
		ts.RmDir(path)
		ts.MkDir(path, 0777)
		return ts, nil
	}
}

// Join a realm/set of machines are already running
func JoinRealm(t *testing.T, realmid string) (*Tstate, error) {
	//fsl, pclnt, err := mkClient("", realmid, []string{""}) // XXX get it from rconfig
	//if err != nil {
	//	return nil, err
	//}
	//rconfig := realm.GetRealmConfig(fsl, realmid)
	db.DFatalf("Unimplemented")
	return nil, nil
}

func bootSystem(t *testing.T, full bool) (*Tstate, error) {
	proc.SetPid(proc.Tpid("test-" + proc.GenPid().String()))
	proc.SetRealm(realmid)
	var s *system.System
	var err error
	if full {
		s, err = system.Boot(realmid, 1, "bootkernelclnt")
	} else {
		s, err = system.BootNamedOnly(realmid, "bootkernelclnt")
	}
	if err != nil {
		return nil, err
	}
	fsl, pclnt := s.GetClnt(0)
	return &Tstate{s, fsl, pclnt, t}, nil
}

func (ts *Tstate) BootNode(n int) error {
	for i := 0; i < n; i++ {
		if err := ts.System.BootNode(realmid, "bootkernelclnt"); err != nil {
			return err
		}
	}
	return nil
}

func (ts *Tstate) NamedAddr() []string {
	return ts.System.GetNamedAddrs()
}

func (ts *Tstate) Shutdown() error {
	return ts.System.Shutdown()
}
