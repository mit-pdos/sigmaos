package test

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

var realmid string // Use this realm to run tests instead of starting a new one. This is used for multi-machine tests.

// Read & set the proc version.
func init() {
	flag.StringVar(&realmid, "realm", "rootrealm", "realm id")
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
	*bootkernelclnt.Kernel
	T       *testing.T
	realmid string
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	b, err := BootPath(t, path)
	if err != nil {
		db.DFatalf("MakeTstatePath: %v\n", err)
	}
	return b
}

func MakeTstate(t *testing.T) *Tstate {
	ts, err := BootKernel(t, "bootkernelclnt/boot.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return ts
}

func MakeTstateAll(t *testing.T) *Tstate {
	ts, err := BootKernel(t, "bootkernelclnt/bootall.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return ts
}

func BootPath(t *testing.T, path string) (*Tstate, error) {
	if path == sp.NAMED {
		return BootKernel(t, "bootkernelclnt/boot.yml")
	} else {
		ts, err := BootKernel(t, "bootkernelclnt/bootall.yml")
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
	return nil, nil
}

func BootKernel(t *testing.T, yml string) (*Tstate, error) {
	k, err := bootkernelclnt.BootKernel(yml)
	if err != nil {
		return nil, err
	}
	os.Setenv(proc.SIGMAREALM, realmid)
	return &Tstate{k, t, realmid}, nil
}

func (ts *Tstate) RunningInRealm() bool {
	return ts.realmid != "rootrealm"
}

func (ts *Tstate) RealmId() string {
	return ts.realmid
}

func (ts *Tstate) NamedAddr() []string {
	return ts.Kernel.NamedAddr()
}

func (ts *Tstate) GetLocalIP() string {
	return ts.GetIP()
}

func (ts *Tstate) Shutdown() error {
	if ts.Kernel != nil {
		return ts.Kernel.Shutdown()
	}
	return nil
}

func (ts *Tstate) BootProcd() error {
	return ts.Boot(sp.PROCDREL)
}

func (ts *Tstate) BootFss3d() error {
	return ts.Boot(sp.S3REL)
}

func (ts *Tstate) BootFsUxd() error {
	return ts.Boot(sp.UXREL)
}
