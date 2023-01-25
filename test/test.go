package test

import (
	"fmt"
	"testing"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/system"
)

const (
	HOSTTMP = "/tmp/sigmaos/"
)

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
	*sigmaclnt.SigmaClnt
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

func bootSystem(t *testing.T, full bool) (*Tstate, error) {
	proc.SetPid(proc.Tpid("test-" + proc.GenPid().String()))
	var s *system.System
	var err error
	if full {
		s, err = system.Boot(1, "bootkernelclnt")
	} else {
		s, err = system.BootNamedOnly("bootkernelclnt")
	}
	if err != nil {
		return nil, err
	}
	sc := s.GetClnt(0)
	return &Tstate{s, sc, t}, nil
}

func (ts *Tstate) BootNode(n int) error {
	for i := 0; i < n; i++ {
		if err := ts.System.BootNode("bootkernelclnt"); err != nil {
			return err
		}
	}
	return nil
}

func (ts *Tstate) NamedAddr() sp.Taddrs {
	return ts.System.GetNamedAddrs()
}

func (ts *Tstate) Shutdown() error {
	return ts.System.Shutdown()
}
