package realm_test

import (
	"log"
	"path"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/linuxsched"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/realm"
)

const (
	SLEEP_TIME_MS = 3000
)

type Tstate struct {
	t        *testing.T
	e        *realm.TestEnv
	cfg      *realm.RealmConfig
	realmFsl *fslib.FsLib
	*fslib.FsLib
	*procclnt.ProcClnt
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	bin := ".."
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	err = ts.e.BootMachined()
	if err != nil {
		t.Fatalf("Boot Machined 2: %v", err)
	}

	db.Name("realm_test")
	ts.realmFsl = fslib.MakeFsLibAddr("realm_test", fslib.Named())
	ts.FsLib = fslib.MakeFsLibAddr("realm_test", cfg.NamedAddr)

	ts.ProcClnt = procclnt.MakeProcClntInit(ts.FsLib, cfg.NamedAddr)

	linuxsched.ScanTopology()

	ts.t = t

	return ts
}

func (ts *Tstate) spawnSpinner() string {
	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, "bin/user/spinner", []string{"name/out_" + pid})
	a.Ncore = proc.Tcore(1)
	err := ts.Spawn(a)
	if err != nil {
		log.Fatalf("Error Spawn in RealmBalanceBenchmark.spawnSpinner: %v", err)
	}
	go func() {
		ts.WaitExit(pid)
	}()
	return pid
}

// Check that the test realm has min <= nMachineds <= max machineds assigned to it
func (ts *Tstate) checkNMachineds(min int, max int) {
	log.Printf("Checking num machineds")
	machineds, err := ts.realmFsl.ReadDir(path.Join(realm.REALMS, realm.TEST_RID))
	if err != nil {
		log.Fatalf("Error ReadDir realm-balance main: %v", err)
	}
	nMachineds := len(machineds)
	ok := assert.True(ts.t, nMachineds >= min && nMachineds <= max, "Wrong number of machineds (x=%v), expected %v <= x <= %v", nMachineds, min, max)
	if !ok {
		debug.PrintStack()
		time.Sleep(100 * time.Second)
	}
}

// Start enough spinning lambdas to fill two Machineds, check that the test
// realm's allocation expanded, evict the spinning lambdas, and check the
// realm's allocation shrank. Then spawn and evict a few more to make sure we
// can still spawn after shrinking.  Assumes other machines in the cluster have
// the same number of cores.
func TestRealmGrowShrink(t *testing.T) {
	ts := makeTstate(t)

	N := int(linuxsched.NCores) / 2

	log.Printf("Starting %v spinning lambdas", N)
	pids := []string{}
	for i := 0; i < N; i++ {
		pids = append(pids, ts.spawnSpinner())
	}

	log.Printf("Sleeping for a bit")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	log.Printf("Starting %v more spinning lambdas", N)
	for i := 0; i < N; i++ {
		pids = append(pids, ts.spawnSpinner())
	}

	log.Printf("Sleeping again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	ts.checkNMachineds(2, 100)

	log.Printf("Evicting %v spinning lambdas", N+7*N/8)
	cnt := 0
	for i := 0; i < N+7*N/8; i++ {
		err := ts.Evict(pids[0])
		assert.Nil(ts.t, err, "Evict")
		log.Printf("Evicted #%v %v", cnt, pids[0])
		cnt += 1
		pids = pids[1:]
	}

	log.Printf("Sleeping yet again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	ts.checkNMachineds(1, 1)

	log.Printf("Starting %v more spinning lambdas", N/2)
	for i := 0; i < int(N/2); i++ {
		pids = append(pids, ts.spawnSpinner())
	}

	log.Printf("Sleeping yet again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	ts.checkNMachineds(1, 100)

	log.Printf("Evicting %v spinning lambdas again", N/2)
	for i := 0; i < int(N/2); i++ {
		ts.Evict(pids[0])
		pids = pids[1:]
	}

	ts.checkNMachineds(1, 1)

	ts.e.Shutdown()
}
