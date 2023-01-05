package test

import (
	"flag"
	"fmt"
	"testing"

	"sigmaos/bootclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/realm"
	sp "sigmaos/sigmap"
)

var realmid string // Use this realm to run tests instead of starting a new one. This is used for multi-machine tests.

// Read & set the proc version.
func init() {
	flag.StringVar(&realmid, "realm", "testrealm", "realm id")
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
	*fslib.FsLib
	*procclnt.ProcClnt
	boot      *bootclnt.Kernel
	kernel    *kernelclnt.KernelClnt
	T         *testing.T
	namedAddr []string
	realmid   string
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	b, err := BootPath(t, path)
	if err != nil {
		db.DFatalf("MakeTstatePath: %v\n", err)
	}
	return b
}

func MakeTstate(t *testing.T) *Tstate {
	b, err := BootKernel(t, realmid, "../bootclnt/boot.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return b
}

func MakeTstateAll(t *testing.T) *Tstate {
	b, err := BootKernel(t, realmid, "../bootclnt/bootall.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return b
}

func BootPath(t *testing.T, path string) (*Tstate, error) {
	if path == sp.NAMED {
		return BootKernel(t, realmid, "../bootclnt/boot.yml")
	} else {
		ts, err := BootKernel(t, realmid, "../bootclnt/bootall.yml")
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
	fsl, pclnt, err := mkClient("", []string{""}) // XXX get it from rconfig
	if err != nil {
		return nil, err
	}
	rconfig := realm.GetRealmConfig(fsl, realmid)
	return &Tstate{fsl, pclnt, nil, nil, t, rconfig.NamedAddrs, realmid}, nil
}

func BootKernel(t *testing.T, realmid, yml string) (*Tstate, error) {
	k, err := bootclnt.BootKernel(realmid, true, yml)
	if err != nil {
		return nil, err
	}
	nameds, err := fslib.SetNamedIP(k.Ip())
	if err != nil {
		return nil, err
	}
	fsl, pclnt, err := mkClient(k.Ip(), nameds)
	if err != nil {
		return nil, err
	}
	kclnt, err := kernelclnt.MakeKernelClnt(fsl, sp.BOOT+"~local/")
	if err != nil {
		return nil, err
	}
	return &Tstate{fsl, pclnt, k, kclnt, t, nameds, realmid}, nil
}

func mkClient(kip string, namedAddr []string) (*fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl, err := fslib.MakeFsLibAddr("test", kip, namedAddr)
	if err != nil {
		return nil, nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, "test", namedAddr)
	return fsl, pclnt, nil
}

func (ts *Tstate) RunningInRealm() bool {
	return ts.realmid != "testrealm"
}

func (ts *Tstate) RealmId() string {
	return ts.realmid
}

func (ts *Tstate) NamedAddr() []string {
	return ts.namedAddr
}

func (ts *Tstate) GetLocalIP() string {
	return ts.boot.Ip()
}

func (ts *Tstate) Shutdown() error {
	if ts.boot != nil {
		return ts.boot.Shutdown()
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

func (ts *Tstate) Boot(s string) error {
	return ts.kernel.Boot(s)
}

func (ts *Tstate) KillOne(s string) error {
	return ts.kernel.Kill(s)
}
