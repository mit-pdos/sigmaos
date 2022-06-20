package realm_test

import (
	"flag"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/realm"
)

const (
	SLEEP_TIME_MS = 3000
)

var version string

// Read & set the proc version.
func init() {
	flag.StringVar(&version, "version", "none", "version")
}

type Tstate struct {
	t        *testing.T
	e        *realm.TestEnv
	cfg      *realm.RealmConfig
	realmFsl *fslib.FsLib
	*fslib.FsLib
	*procclnt.ProcClnt
}

func makeTstate(t *testing.T) *Tstate {
	setVersion()
	ts := &Tstate{}
	e := realm.MakeTestEnv(np.TEST_RID)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	err = ts.e.BootMachined()
	if err != nil {
		t.Fatalf("Boot Noded 2: %v", err)
	}

	program := "realm_test"
	ts.realmFsl = fslib.MakeFsLibAddr(program, fslib.Named())
	ts.FsLib = fslib.MakeFsLibAddr(program, cfg.NamedAddrs)

	ts.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), ts.FsLib, program, cfg.NamedAddrs)

	linuxsched.ScanTopology()

	ts.t = t

	return ts
}

func setVersion() {
	if version == "" || version == "none" || !flag.Parsed() {
		db.DFatalf("Version not set in test")
	}
	proc.Version = version
}

func (ts *Tstate) spawnSpinner() proc.Tpid {
	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, "user/spinner", []string{"name/"})
	a.Ncore = proc.Tcore(1)
	err := ts.Spawn(a)
	if err != nil {
		db.DFatalf("Error Spawn in RealmBalanceBenchmark.spawnSpinner: %v", err)
	}
	go func() {
		ts.WaitExit(pid)
	}()
	return pid
}

// Check that the test realm has min <= nNodeds <= max nodeds assigned to it
func (ts *Tstate) checkNNodeds(min int, max int) {
	db.DPrintf("TEST", "Checking num nodeds")
	cfg := realm.GetRealmConfig(ts.realmFsl, np.TEST_RID)
	nNodeds := len(cfg.NodedsActive)
	db.DPrintf("TEST", "Done Checking num nodeds")
	ok := assert.True(ts.t, nNodeds >= min && nNodeds <= max, "Wrong number of nodeds (x=%v), expected %v <= x <= %v", nNodeds, min, max)
	if !ok {
		debug.PrintStack()
	}
}

func TestStartStop(t *testing.T) {
	ts := makeTstate(t)
	ts.checkNNodeds(1, 1)
	ts.e.Shutdown()
}

// Start enough spinning lambdas to fill two Nodeds, check that the test
// realm's allocation expanded, then exit.
func TestRealmGrow(t *testing.T) {
	ts := makeTstate(t)

	N := int(linuxsched.NCores) / 2

	db.DPrintf("TEST", "Starting %v spinning lambdas", N)
	pids := []proc.Tpid{}
	for i := 0; i < N; i++ {
		pids = append(pids, ts.spawnSpinner())
	}

	db.DPrintf("TEST", "Sleeping for a bit")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	db.DPrintf("TEST", "Starting %v more spinning lambdas", N)
	for i := 0; i < N; i++ {
		pids = append(pids, ts.spawnSpinner())
	}

	db.DPrintf("TEST", "Sleeping again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	ts.checkNNodeds(2, 100)

	ts.e.Shutdown()
}

// Start enough spinning lambdas to fill two Nodeds, check that the test
// realm's allocation expanded, evict the spinning lambdas, and check the
// realm's allocation shrank. Then spawn and evict a few more to make sure we
// can still spawn after shrinking.  Assumes other machines in the cluster have
// the same number of cores.
func TestRealmShrink(t *testing.T) {
	ts := makeTstate(t)

	N := int(linuxsched.NCores) / 2

	db.DPrintf("TEST", "Starting %v spinning lambdas", N)
	pids := []proc.Tpid{}
	for i := 0; i < N; i++ {
		pids = append(pids, ts.spawnSpinner())
	}

	db.DPrintf("TEST", "Sleeping for a bit")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	db.DPrintf("TEST", "Starting %v more spinning lambdas", N)
	for i := 0; i < N; i++ {
		pids = append(pids, ts.spawnSpinner())
	}

	db.DPrintf("TEST", "Sleeping again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	ts.checkNNodeds(2, 100)

	db.DPrintf("TEST", "Creating a new realm to contend with the old one")
	// Create another realm to contend with this one.
	ts.e.CreateRealm("2000")

	db.DPrintf("TEST", "Sleeping yet again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	ts.checkNNodeds(1, 1)

	db.DPrintf("TEST", "Destroying the new, contending realm")
	ts.e.DestroyRealm("2000")

	db.DPrintf("TEST", "Starting %v more spinning lambdas", N/2)
	for i := 0; i < int(N); i++ {
		pids = append(pids, ts.spawnSpinner())
	}

	db.DPrintf("TEST", "Sleeping yet again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	ts.checkNNodeds(2, 100)

	ts.e.Shutdown()
}
