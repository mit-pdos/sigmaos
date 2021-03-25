package perf

import (
	"errors"
	"log"
	"strconv"
	"time"

	// db "ulambda/debug"
	"ulambda/fslib"
)

type SpinTestStarter struct {
	nSpinners int
	dim       string
	its       string
	*fslib.FsLib
}

func (s *SpinTestStarter) spawnSpinnerWithPid(pid string) {
	a := &fslib.Attr{pid, "bin/perf-spinner", "", []string{s.dim, s.its}, nil, nil, nil}
	err := s.Spawn(a)
	if err != nil {
		log.Fatalf("couldn't spawn %v: %v\n", pid, err)
	}
}

func (s *SpinTestStarter) spawnSpinner() string {
	pid := fslib.GenPid()
	s.spawnSpinnerWithPid(pid)
	return pid
}

func MakeSpinTestStarter(args []string) (*SpinTestStarter, error) {
	if len(args) < 3 {
		return nil, errors.New("MakeSpinTestStarter: too few arguments")
	}
	log.Printf("MakeSpinTestStarter: %v\n", args)

	s := &SpinTestStarter{}
	s.FsLib = fslib.MakeFsLib("spin-test-starter")

	nSpinners, err := strconv.Atoi(args[0])
	s.nSpinners = nSpinners
	if err != nil {
		log.Fatalf("Invalid dimension: %v, %v\n", args[0], err)
	}

	_, err = strconv.Atoi(args[1])
	s.dim = args[1]
	if err != nil {
		log.Fatalf("Invalid dimension: %v, %v\n", args[1], err)
	}

	_, err = strconv.Atoi(args[2])
	s.its = args[2]
	if err != nil {
		log.Fatalf("Invalid num interations: %v, %v\n", args[2], err)
	}

	return s, nil
}

func (s *SpinTestStarter) Work() {
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
		s.spawnSpinnerWithPid(pid)
	}

	// Wait for them all
	for pid, _ := range pids {
		s.Wait(pid)
	}
	end := time.Now()

	// Calculate elapsed time
	elapsed := end.Sub(start)
	log.Printf("Elapsed time: %f usec(s)\n", float64(elapsed.Microseconds()))
}
