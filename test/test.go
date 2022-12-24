package test

import (
	"flag"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernel"
	"sigmaos/linuxsched"
	"sigmaos/proc"
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

type Tstate struct {
	sync.Mutex
	realmid string
	wg      sync.WaitGroup
	T       *testing.T
	*kernel.System
	replicas  []*kernel.System
	namedAddr []string
}

func makeTstate(t *testing.T, realmid string) *Tstate {
	setVersion()
	ts := &Tstate{}
	ts.T = t
	ts.realmid = realmid
	ts.namedAddr = fslib.Named()
	return ts
}

func MakeTstatePath(t *testing.T, path string) *Tstate {
	if path == sp.NAMED {
		return MakeTstate(t)
	} else {
		ts := MakeTstateAll(t)
		ts.RmDir(path)
		ts.MkDir(path, 0777)
		return ts
	}
}

type Bstate struct {
	*fslib.FsLib
	kernel *kernel.Kernel
	T      *testing.T
}

func BootKernel(t *testing.T, contain bool) (*Bstate, error) {
	k, err := kernel.BootKernel(contain)
	if err != nil {
		return nil, err
	}
	fslib, err := fslib.MakeFsLibAddr("test", fslib.Named())
	if err != nil {
		return nil, err
	}
	return &Bstate{fslib, k, t}, nil
}

func (bs *Bstate) Shutdown() error {
	return bs.kernel.Shutdown()
}

func MakeTstate(t *testing.T) *Tstate {
	ts := makeTstate(t, "")
	ts.makeSystem(kernel.MakeSystemNamed)
	return ts
}

// A realm/set of machines are already running
func MakeTstateRealm(t *testing.T, realmid string) *Tstate {
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

func MakeTstateAll(t *testing.T) *Tstate {
	var ts *Tstate
	// If no realm is running (single-machine)
	if realmid == "" {
		ts = makeTstate(t, realmid)
		ts.makeSystem(kernel.MakeSystemAll)
	} else {
		ts = MakeTstateRealm(t, realmid)
	}
	return ts
}

func (ts *Tstate) RunningInRealm() bool {
	return ts.realmid != ""
}

func (ts *Tstate) RealmId() string {
	return ts.realmid
}

func (ts *Tstate) NamedAddr() []string {
	return ts.namedAddr
}

func (ts *Tstate) Shutdown() {
	db.DPrintf(db.TEST, "Shutting down")
	ts.System.Shutdown()
	for _, r := range ts.replicas {
		r.Shutdown()
	}
	N := 200 // Crashing procds in mr test leave several fids open; maybe too many?
	assert.True(ts.T, ts.PathClnt.FidClnt.Len() < N, "Too many FIDs open (%v): %v", ts.PathClnt.FidClnt.Len(), ts.PathClnt.FidClnt)
	db.DPrintf(db.TEST, "Done shutting down")
}

func (ts *Tstate) addNamedReplica(i int) error {
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

func (ts *Tstate) startReplicas() {
	ts.replicas = []*kernel.System{}
	// Start additional replicas
	for i := 0; i < len(fslib.Named())-1; i++ {
		// Needs to happen in a separate thread because MakeSystemNamed will block until the replicas are able to process requests.
		go ts.addNamedReplica(i + 1)
	}
}

func (ts *Tstate) makeSystem(mkSys func(string, string, int, *sessp.Tinterval) (*kernel.System, error)) error {
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
