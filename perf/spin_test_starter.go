package perf

import (
	"errors"
	"log"
	"strconv"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
)

type SpinTestStarter struct {
	nSpinners int
	*fslib.FsLib
}

func (s *SpinTestStarter) spawnSpinnerWithPid(pid string, msecs string) {
	a := &fslib.Attr{pid, "bin/spinner", "", []string{msecs}, nil, nil, nil}
	err := s.Spawn(a)
	if err != nil {
		log.Fatalf("couldn't spawn %v: %v\n", pid, err)
	}
}

func (s *SpinTestStarter) spawnSpinner(msecs string) string {
	pid := fslib.GenPid()
	s.spawnSpinnerWithPid(pid, msecs)
	return pid
}

func MakeSpinTestStarter(args []string) (*SpinTestStarter, error) {
	if len(args) != 1 {
		return nil, errors.New("MakeSpinTestStarter: too few arguments")
	}
	log.Printf("MakeSpinTestStarter: %v\n", args)
	db.SetDebug(false)

	s := &SpinTestStarter{}
	s.FsLib = fslib.MakeFsLib("spin-test-starter")
	nSpinners, err := strconv.Atoi(args[0])
	s.nSpinners = nSpinners
	if err != nil {
		log.Fatalf("Invalid number of spinners: %v, %v\n", args[0], err)
	}

	db.SetDebug(false)

	return s, nil
}

func (s *SpinTestStarter) Work() {
	// Test params
	spinMsecs := 2000

	msecs := strconv.Itoa(spinMsecs)
	pids := map[string]int{}

	// Gen pids
	for i := 0; i < s.nSpinners; i++ {
		pid := fslib.GenPid()
		_, alreadySpawned := pids[pid]
		for alreadySpawned {
			pid = fslib.GenPid()
			_, alreadySpawned = pids[pid]
		}
		pids[pid] = i
	}

	start := time.Now()
	// Start some lambdas
	for pid, _ := range pids {
		s.spawnSpinnerWithPid(pid, msecs)
	}

	// Wait for them all
	for pid, _ := range pids {
		s.Wait(pid)
	}
	end := time.Now()

	// Calculate elapsed time
	elapsed := end.Sub(start)
	log.Printf("Elapsed time: %v msec(s)\n", elapsed.Milliseconds())
}
