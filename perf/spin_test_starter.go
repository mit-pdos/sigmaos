package perf

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	// db "ulambda/debug"
	"ulambda/fslib"
)

type SpinTestStarter struct {
	nSpinners int
	dim       string
	its       string
	native    bool
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
	if len(args) < 4 {
		return nil, errors.New("MakeSpinTestStarter: too few arguments")
	}
	log.Printf("MakeSpinTestStarter: %v\n", args)

	s := &SpinTestStarter{}
	if args[3] == "native" {
		s.native = true
	} else if args[3] == "9p" {
		s.native = false
	} else {
		return nil, errors.New("MakeSpinTestStarter: invalid test type")
	}

	if !s.native {
		s.FsLib = fslib.MakeFsLib("spin-test-starter")
	}

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

func (s *SpinTestStarter) TestUlambda() time.Duration {
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
	return elapsed
}

func (s *SpinTestStarter) TestNative() time.Duration {
	pids := map[string]*exec.Cmd{}

	// Gen pids
	for i := 0; i < s.nSpinners; i++ {
		pid := fslib.GenPid()
		_, alreadySpawned := pids[pid]
		for alreadySpawned {
			pid = fslib.GenPid()
			_, alreadySpawned = pids[pid]
		}

		// Set up command struct
		cmd := exec.Command("./bin/perf-spinner", []string{pid, s.dim, s.its, "native"}...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Store it along
		pids[pid] = cmd
	}

	start := time.Now()
	// Start some lambdas
	for pid, _ := range pids {
		err := pids[pid].Start()
		if err != nil {
			log.Printf("Error starting pid: %v, %v\n", pid, err)
		}
	}

	// Wait for them all
	for pid, _ := range pids {
		err := pids[pid].Wait()
		if err != nil {
			log.Printf("Error in pid %v exit: %v", pid, err)
		}
	}
	end := time.Now()

	// Calculate elapsed time
	elapsed := end.Sub(start)
	return elapsed
}

func (s *SpinTestStarter) Work() {
	if s.native {
		nativeElapsed := s.TestNative()
		log.Printf("Native elapsed time: %f usec(s)\n", float64(nativeElapsed.Microseconds()))
	} else {
		ulambdaElapsed := s.TestUlambda()
		log.Printf("Ulambda elapsed time: %f usec(s)\n", float64(ulambdaElapsed.Microseconds()))
	}
}
