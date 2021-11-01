package benchmarks

import (
	"log"
	"path"
	"time"

	"ulambda/fslib"
	"ulambda/linuxsched"
	"ulambda/proc"
	"ulambda/procinit"
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
	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})
	b.ProcClnt = procinit.MakeProcClnt(b.FsLib, procinit.GetProcLayersMap())
	linuxsched.ScanTopology()
	return b
}

func (b *RealmBalanceBenchmark) spawnSpinner() string {
	pid := proc.GenPid()
	a := proc.MakeProc(pid, "bin/user/spinner", []string{"name/out_" + pid})
	a.Ncore = proc.Tcore(1)
	err := b.Spawn(a)
	if err != nil {
		log.Fatalf("Error Spawn in RealmBalanceBenchmark.spawnSpinner: %v", err)
	}
	return pid
}

// Check that the test realm has min <= nRealmds <= max realmds assigned to it
func (b *RealmBalanceBenchmark) checkNRealmds(min int, max int) {
	log.Printf("Checking num realmds")
	realmds, err := b.realmFsl.ReadDir(path.Join(realm.REALMS, realm.TEST_RID))
	if err != nil {
		log.Fatalf("Error ReadDir realm-balance main: %v", err)
	}
	nRealmds := len(realmds)
	log.Printf("# realmds: %v", nRealmds)
	if nRealmds >= min && nRealmds <= max {
		log.Printf("Correct num realmds: %v <= %v <= %v", min, nRealmds, max)
	} else {
		log.Fatalf("FAIL: %v realmds, expected %v <= x <= %v", nRealmds, min, max)
	}
}

// Start enough spinning lambdas to fill two Realmds, check that the test
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

	b.checkNRealmds(2, 100)

	log.Printf("Evicting %v spinning lambdas", linuxsched.NCores+7*linuxsched.NCores/8)
	for i := 0; i < int(linuxsched.NCores)+7*int(linuxsched.NCores)/8; i++ {
		b.Evict(pids[0])
		pids = pids[1:]
	}

	log.Printf("Sleeping yet again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	b.checkNRealmds(1, 1)

	log.Printf("Starting %v more spinning lambdas", linuxsched.NCores/2)
	for i := 0; i < int(linuxsched.NCores/2); i++ {
		pids = append(pids, b.spawnSpinner())
	}

	log.Printf("Sleeping yet again")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)

	b.checkNRealmds(1, 100)

	log.Printf("Evicting %v spinning lambdas again", linuxsched.NCores/2)
	for i := 0; i < int(linuxsched.NCores/2); i++ {
		b.Evict(pids[0])
		pids = pids[1:]
	}

	b.checkNRealmds(1, 1)

	log.Printf("PASS")
}
