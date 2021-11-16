package benchmarks

import (
	"log"
	"path"
	"time"

	"ulambda/fslib"
	"ulambda/linuxsched"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/realm"
)

const (
	SLEEP_TIME_MS = 3000
)

type RealmBalanceBenchmark struct {
	realmFsl *fslib.FsLib
	*fslib.FsLib
	proc.ProcClnt
}

func MakeRealmBalanceBenchmark(realmFsl *fslib.FsLib, fsl *fslib.FsLib) *RealmBalanceBenchmark {
	b := &RealmBalanceBenchmark{}
	b.realmFsl = realmFsl
	b.FsLib = fsl
	b.ProcClnt = procclnt.MakeProcClnt(b.FsLib)
	linuxsched.ScanTopology()
	return b
}

func (b *RealmBalanceBenchmark) spawnSpinner() string {
	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, "bin/user/spinner", []string{"name/out_" + pid})
	a.Ncore = proc.Tcore(1)
	err := b.Spawn(a)
	if err != nil {
		log.Fatalf("Error Spawn in RealmBalanceBenchmark.spawnSpinner: %v", err)
	}
	return pid
}

// Check that the test realm has min <= nMachineds <= max machineds assigned to it
func (b *RealmBalanceBenchmark) checkNMachineds(min int, max int) {
	log.Printf("Checking num machineds")
	machineds, err := b.realmFsl.ReadDir(path.Join(realm.REALMS, realm.TEST_RID))
	if err != nil {
		log.Fatalf("Error ReadDir realm-balance main: %v", err)
	}
	nMachineds := len(machineds)
	log.Printf("# machineds: %v", nMachineds)
	if nMachineds >= min && nMachineds <= max {
		log.Printf("Correct num machineds: %v <= %v <= %v", min, nMachineds, max)
	} else {
		log.Fatalf("FAIL: %v machineds, expected %v <= x <= %v", nMachineds, min, max)
	}
}

// Start enough spinning lambdas to fill two Machineds, check that the test
// realm's allocation expanded, evict the spinning lambdas, and check the
// realm's allocation shrank. Then spawn and evict a few more to make sure we
// can still spawn after shrinking.  Assumes other machines in the cluster have
// the same number of cores.
func (b *RealmBalanceBenchmark) Run() {
	log.Printf("Starting RealmBalanceBenchmark...")

	log.Printf("Starting %v spinning lambdas", linuxsched.NCores)
	pids := []string{}
	for i := 0; i < int(linuxsched.NCores); i++ {
		pids = append(pids, b.spawnSpinner())
	}

	log.Printf("Sleeping for a bit")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	log.Printf("Starting %v more spinning lambdas", linuxsched.NCores)
	for i := 0; i < int(linuxsched.NCores); i++ {
		pids = append(pids, b.spawnSpinner())
	}

	log.Printf("Sleeping again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	b.checkNMachineds(2, 100)

	log.Printf("Evicting %v spinning lambdas", linuxsched.NCores+7*linuxsched.NCores/8)
	for i := 0; i < int(linuxsched.NCores)+7*int(linuxsched.NCores)/8; i++ {
		b.Evict(pids[0])
		pids = pids[1:]
	}

	log.Printf("Sleeping yet again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	b.checkNMachineds(1, 1)

	log.Printf("Starting %v more spinning lambdas", linuxsched.NCores/2)
	for i := 0; i < int(linuxsched.NCores/2); i++ {
		pids = append(pids, b.spawnSpinner())
	}

	log.Printf("Sleeping yet again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	b.checkNMachineds(1, 100)

	log.Printf("Evicting %v spinning lambdas again", linuxsched.NCores/2)
	for i := 0; i < int(linuxsched.NCores/2); i++ {
		b.Evict(pids[0])
		pids = pids[1:]
	}

	b.checkNMachineds(1, 1)

	log.Printf("PASS")
}
