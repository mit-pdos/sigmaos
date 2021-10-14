package benchmarks

import (
	"log"
	"time"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procinit"
)

const (
	N_SPINNERS_PER_PROCD = 40
	SLEEP_TIME_MS        = 3000
)

type RealmBalanceBenchmark struct {
	*fslib.FsLib
	proc.ProcClnt
}

func MakeRealmBalanceBenchmark(fsl *fslib.FsLib) *RealmBalanceBenchmark {
	b := &RealmBalanceBenchmark{}
	b.FsLib = fsl
	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})
	b.ProcClnt = procinit.MakeProcClnt(b.FsLib, procinit.GetProcLayersMap())
	return b
}

func (b *RealmBalanceBenchmark) spawnSpinner() {
	pid := proc.GenPid()
	a := &proc.Proc{pid, "bin/user/spinner", "",
		[]string{"name/out_" + pid},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.Tcore(1),
	}
	err := b.Spawn(a)
	if err != nil {
		log.Fatalf("Error Spawn in RealmBalanceBenchmark.spawnSpinner: %v", err)
	}
}

func (b *RealmBalanceBenchmark) Run() {
	log.Printf("Starting RealmBalanceBenchmark...")
	for i := 0; i < N_SPINNERS_PER_PROCD; i++ {
		b.spawnSpinner()
	}
	log.Printf("RealmBalanceBenchmark sleeping...")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)
	for i := 0; i < N_SPINNERS_PER_PROCD; i++ {
		b.spawnSpinner()
	}
	log.Printf("RealmBalanceBenchmark done spawning...")
	time.Sleep(SLEEP_TIME_MS * time.Millisecond)
	log.Printf("RealmBalanceBenchmark checking num realmds...")
}
