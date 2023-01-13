package test

import (
	"flag"
	"fmt"
	"os"
	"testing"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/realmv1"
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
	*realmv1.Realm
	T *testing.T
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	b, err := BootPath(t, path)
	if err != nil {
		db.DFatalf("MakeTstatePath: %v\n", err)
	}
	return b
}

func MakeTstate(t *testing.T) *Tstate {
	b, err := BootRealm(t, "bootkernelclnt/boot.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return b
}

func MakeTstateAll(t *testing.T) *Tstate {
	b, err := BootRealm(t, "bootkernelclnt/bootall.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return b
}

func BootPath(t *testing.T, path string) (*Tstate, error) {
	if path == sp.NAMED {
		return BootRealm(t, "bootkernelclnt/boot.yml")
	} else {
		ts, err := BootRealm(t, "bootkernelclnt/bootall.yml")
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

func BootRealm(t *testing.T, yml string) (*Tstate, error) {
	r, err := realmv1.BootRealm(yml)
	if err != nil {
		return nil, err
	}
	os.Setenv(proc.SIGMAREALM, realmid)
	return &Tstate{r, t}, nil
}

func BootRealmOld(t *testing.T, realmid, yml string) (*Tstate, error) {
	r, err := realmv1.BootRealmOld(realmid, yml)
	if err != nil {
		return nil, err
	}
	os.Setenv(proc.SIGMAREALM, realmid)
	return &Tstate{r, t}, nil
}

func (ts *Tstate) RunningInRealm() bool {
	return ts.Realmid != "rootrealm"
}

func (ts *Tstate) RealmId() string {
	return ts.Realmid
}

func (ts *Tstate) NamedAddr() []string {
	return ts.Realm.NamedAddr()
}

func (ts *Tstate) GetLocalIP() string {
	return ts.Realm.GetIP()
}

func (ts *Tstate) ShutdownOld() error {
	if ts.Realm != nil {
		return ts.Realm.ShutdownOld()
	}
	return nil
}

func (ts *Tstate) Shutdown() error {
	if ts.Realm != nil {
		return ts.Realm.Shutdown()
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
