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

var version string
var realmid string // Use this realm to run tests instead of starting a new one. This is used for multi-machine tests.

// Read & set the proc version.
func init() {
	flag.StringVar(&version, "version", "none", "version")
	flag.StringVar(&realmid, "realm", "test-realm", "realm id")
}

func setVersion() {
	if version == "" || version == "none" || !flag.Parsed() {
		db.DFatalf("Version not set in test")
	}
	proc.Version = version
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
	b, err := BootKernel(t, realmid, "boot.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return b
}

func MakeTstateAll(t *testing.T) *Tstate {
	b, err := BootKernel(t, realmid, "bootall.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return b
}

func BootPath(t *testing.T, path string) (*Tstate, error) {
	if path == sp.NAMED {
		return BootKernel(t, realmid, "boot.yml")
	} else {
		ts, err := BootKernel(t, realmid, "bootall.yml")
		if err != nil {
			return nil, err
		}
		ts.RmDir(path)
		ts.MkDir(path, 0777)
		return ts, nil
	}
}

func mkClient() (*fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl, err := fslib.MakeFsLibAddr("test", fslib.Named())
	if err != nil {
		return nil, nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, "test", fslib.Named())
	return fsl, pclnt, nil
}

// Join a realm/set of machines are already running
func JoinRealm(t *testing.T, realmid string) (*Tstate, error) {
	fsl, pclnt, err := mkClient()
	if err != nil {
		return nil, err
	}
	rconfig := realm.GetRealmConfig(fsl, realmid)
	return &Tstate{fsl, pclnt, nil, nil, t, rconfig.NamedAddrs, realmid}, nil
}

func BootKernel(t *testing.T, realmid, yml string) (*Tstate, error) {
	setVersion()
	b, err := bootclnt.BootKernel(realmid, false, yml)
	if err != nil {
		return nil, err
	}
	fsl, pclnt, err := mkClient()
	if err != nil {
		return nil, err
	}
	kclnt, err := kernelclnt.MakeKernelClnt(fsl, sp.BOOT+"~local/")
	if err != nil {
		return nil, err
	}

	return &Tstate{fsl, pclnt, b, kclnt, t, fslib.Named(), realmid}, nil
}

func (ts *Tstate) RunningInRealm() bool {
	return ts.realmid != "test-realm"
}

func (ts *Tstate) RealmId() string {
	return ts.realmid
}

func (ts *Tstate) NamedAddr() []string {
	return ts.namedAddr
}

func (ts *Tstate) Shutdown() error {
	if ts.boot != nil {
		return ts.boot.Shutdown()
	}
	return nil
}

func (ts *Tstate) BootProcd() error {
	return ts.kernel.Boot("procd")
}

func (ts *Tstate) BootFss3d() error {
	return ts.kernel.Boot("fss3d")
}

func (ts *Tstate) BootFsUxd() error {
	return ts.kernel.Boot("fsuxd")
}

func (ts *Tstate) KillOne(s string) error {
	return ts.kernel.Kill(s)
}
