package test

import (
	"flag"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/bootclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernel"
	"sigmaos/kernelclnt"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/realm"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

var version string
var realmid string // Use this realm to run tests instead of starting a new one. This is used for multi-machine tests.

// Read & set the proc version.
func init() {
	flag.StringVar(&version, "version", "none", "version")
	flag.StringVar(&realmid, "realm", "", "realm id")
}

type Tstate1 struct {
	sync.Mutex
	realmid string
	wg      sync.WaitGroup
	T       *testing.T
	*kernel.System
	replicas  []*kernel.System
	namedAddr []string
}

func makeTstate(t *testing.T, realmid string) *Tstate1 {
	setVersion()
	ts := &Tstate1{}
	ts.T = t
	ts.realmid = realmid
	ts.namedAddr = fslib.Named()
	return ts
}

func MakeTstatePath1(t *testing.T, path string) *Tstate1 {
	if path == sp.NAMED {
		return MakeTstate1(t)
	} else {
		ts := MakeTstateAll1(t)
		ts.RmDir(path)
		ts.MkDir(path, 0777)
		return ts
	}
}

func MakeTstate1(t *testing.T) *Tstate1 {
	ts := makeTstate(t, "")
	ts.makeSystem(kernel.MakeSystemNamed)
	return ts
}

func MakeTstateAll1(t *testing.T) *Tstate1 {
	var ts *Tstate1
	// If no realm is running (single-machine)
	if realmid == "" {
		ts = makeTstate(t, realmid)
		ts.makeSystem(kernel.MakeSystemAll)
	} else {
		ts = MakeTstateRealm(t, realmid)
	}
	return ts
}

// A realm/set of machines are already running
func MakeTstateRealm(t *testing.T, realmid string) *Tstate1 {
	ts := makeTstate(t, realmid)
	// XXX make fslib exit?
	fsl, err := fslib.MakeFsLib("test")
	if err != nil {
		return nil
	}
	rconfig := realm.GetRealmConfig(fsl, realmid)
	ts.namedAddr = rconfig.NamedAddrs
	sys, err := kernel.MakeSystem("test", realmid, rconfig.NamedAddrs, sessp.MkInterval(0, uint64(linuxsched.NCores)))
	if err != nil {
		return nil
	}
	ts.System = sys
	return ts
}

func (ts *Tstate1) RunningInRealm() bool {
	return ts.realmid != ""
}

func (ts *Tstate1) RealmId() string {
	return ts.realmid
}

func (ts *Tstate1) NamedAddr() []string {
	return ts.namedAddr
}

func (ts *Tstate1) Shutdown() {
	db.DPrintf(db.TEST, "Shutting down")
	ts.System.Shutdown()
	for _, r := range ts.replicas {
		r.Shutdown()
	}
	N := 200 // Crashing procds in mr test leave several fids open; maybe too many?
	assert.True(ts.T, ts.PathClnt.FidClnt.Len() < N, "Too many FIDs open (%v): %v", ts.PathClnt.FidClnt.Len(), ts.PathClnt.FidClnt)
	db.DPrintf(db.TEST, "Done shutting down")
}

func (ts *Tstate1) addNamedReplica(i int) error {
	defer ts.wg.Done()
	r, err := kernel.MakeSystemNamed("test", sp.TEST_RID, i, sessp.MkInterval(0, uint64(linuxsched.NCores)))
	if err != nil {
		return err
	}
	ts.Lock()
	defer ts.Unlock()
	ts.replicas = append(ts.replicas, r)
	return nil
}

func (ts *Tstate1) startReplicas() {
	ts.replicas = []*kernel.System{}
	// Start additional replicas
	for i := 0; i < len(fslib.Named())-1; i++ {
		// Needs to happen in a separate thread because MakeSystemNamed will block until the replicas are able to process requests.
		go ts.addNamedReplica(i + 1)
	}
}

func (ts *Tstate1) makeSystem(mkSys func(string, string, int, *sessp.Tinterval) (*kernel.System, error)) error {
	ts.wg.Add(len(fslib.Named()))
	// Needs to happen in a separate thread because MakeSystem will block until enough replicas have started (if named is replicated).
	var err error
	go func() {
		defer ts.wg.Done()
		sys, r := mkSys("test", sp.TEST_RID, 0, sessp.MkInterval(0, uint64(linuxsched.NCores)))
		if r != nil {
			err = r
		} else {
			ts.System = sys
		}
	}()
	ts.startReplicas()
	ts.wg.Wait()
	return err
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

//
// New implementation of API
//

type Tstate = Bstate

func MakeTstatePath(t *testing.T, path string) *Bstate {
	b, err := MakeBootPath(t, path)
	if err != nil {
		db.DFatalf("MakeTstatePath: %v\n", err)
	}
	return b
}

func MakeTstate(t *testing.T) *Bstate {
	b, err := BootKernel(t, "boot.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return b
}

func MakeTstateAll(t *testing.T) *Bstate {
	b, err := BootKernel(t, "bootall.yml")
	if err != nil {
		db.DFatalf("MakeTstate: %v\n", err)
	}
	return b
}

type Bstate struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	boot      *bootclnt.Kernel
	kernel    *kernelclnt.KernelClnt
	T         *testing.T
	namedAddr []string
}

func MakeBootPath(t *testing.T, path string) (*Bstate, error) {
	if path == sp.NAMED {
		return BootKernel(t, "boot.yml")
	} else {
		bs, err := BootKernel(t, "bootall.yml")
		if err != nil {
			return nil, err
		}
		bs.RmDir(path)
		bs.MkDir(path, 0777)
		return bs, nil
	}
}

func BootKernel(t *testing.T, yml string) (*Bstate, error) {
	setVersion()
	b, err := bootclnt.BootKernel(false, yml)
	if err != nil {
		return nil, err
	}
	fsl, err := fslib.MakeFsLibAddr("test", fslib.Named())
	if err != nil {
		return nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, "test", fslib.Named())
	kclnt, err := kernelclnt.MakeKernelClnt(fsl, sp.BOOT+"~local/")
	if err != nil {
		return nil, err
	}
	return &Bstate{fsl, pclnt, b, kclnt, t, fslib.Named()}, nil
}

func (bs *Bstate) NamedAddr() []string {
	return bs.namedAddr
}

func (bs *Bstate) Shutdown() error {
	return bs.boot.Shutdown()
}

func (bs *Bstate) BootProcd() error {
	return bs.kernel.Boot("procd")
}

func (bs *Bstate) KillOne(s string) error {
	return bs.kernel.Kill(s)
}
