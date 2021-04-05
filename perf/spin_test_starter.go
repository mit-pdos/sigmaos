package perf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	baseline  bool
	aws       bool
	local     bool
	*fslib.FsLib
}

func (s *SpinTestStarter) spawnSpinnerWithPid(pid string) {
	a := &fslib.Attr{pid, "bin/c-spinner", "", []string{s.dim, s.its}, nil, nil, nil}
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
	if len(args) < 5 {
		return nil, errors.New("MakeSpinTestStarter: too few arguments")
	}
	log.Printf("MakeSpinTestStarter: %v\n", args)

	s := &SpinTestStarter{}
	if args[3] == "native" {
		s.native = true
	} else if args[3] == "9p" {
		s.native = false
	} else if args[3] == "aws" {
		s.aws = true
	} else if args[3] == "baseline" {
		s.baseline = true
	} else {
		return nil, errors.New("MakeSpinTestStarter: invalid test type")
	}

	if args[4] == "local" {
		s.local = true
	}

	if s.local && s.aws {
		log.Fatalf("Can't run as local & aws\n")
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

	i, err := strconv.Atoi(args[2])
	// Limited by the AWS API Gateway timeout
	if i > 5000 && s.local == false {
		s.its = "5000"
	} else {
		s.its = args[2]
	}
	if err != nil {
		log.Fatalf("Invalid num interations: %v, %v\n", args[2], err)
	}

	return s, nil
}

func (s *SpinTestStarter) TestNinep() time.Duration {
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
		cmd := exec.Command("./bin/c-spinner", []string{pid, s.dim, s.its, "native"}...)
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

func (s *SpinTestStarter) TestAws() time.Duration {
	vals := map[string]bool{"baseline": true}
	body, err := json.Marshal(vals)

	if err != nil {
		log.Fatalf("Error marshalling for lamda baseline: %v", err)
	}

	url := fmt.Sprintf("https://m5ica91644.execute-api.us-east-1.amazonaws.com/default/cpp-spin?dim=%v&its=%v", s.dim, s.its)

	start := time.Now()
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	end := time.Now()

	if resp.StatusCode != 200 {
		log.Fatalf("Bad response code: %v, %v", resp.StatusCode, resp)
	}

	if err != nil {
		log.Fatalf("Error in HTTP POST: %v", err)
	}

	// Calculate elapsed time
	elapsed := end.Sub(start)
	return elapsed
}

func (s *SpinTestStarter) TestLocalBaseline() {
	pid := fslib.GenPid()

	// Set up command struct
	cmd := exec.Command("./bin/c-spinner", []string{pid, s.dim, s.its, "baseline"}...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start some lambdas
	err := cmd.Start()
	if err != nil {
		log.Printf("Error starting pid: %v, %v\n", pid, err)
	}

	// Wait for them all
	err = cmd.Wait()
	if err != nil {
		log.Printf("Error in pid %v exit: %v", pid, err)
	}
}

func (s *SpinTestStarter) TestAwsBaseline() {
	vals := map[string]bool{"baseline": true}
	body, err := json.Marshal(vals)

	if err != nil {
		log.Fatalf("Error marshalling for lamda baseline: %v", err)
	}

	url := fmt.Sprintf("https://m5ica91644.execute-api.us-east-1.amazonaws.com/default/cpp-spin?dim=%v&its=%v", s.dim, s.its)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))

	if resp.StatusCode != 200 {
		log.Fatalf("Bad response code: %v, %v", resp.StatusCode, resp)
	}

	if err != nil {
		log.Fatalf("Error in HTTP POST: %v", err)
	}

	var res map[string]interface{}

	json.NewDecoder(resp.Body).Decode(&res)

	if err != nil {
		log.Fatalf("Error unmarshalling response body: %v", err)
	}

	log.Printf("%v", res["message"])
}

func (s *SpinTestStarter) Work() {
	if s.baseline {
		if s.local {
			s.TestLocalBaseline()
		} else {
			s.TestAwsBaseline()
		}
	} else if !s.local {
		awsElapsed := s.TestAws()
		log.Printf("Aws elapsed time: %f usec(s)\n", float64(awsElapsed.Microseconds()))
	} else if s.native {
		nativeElapsed := s.TestNative()
		log.Printf("Native elapsed time: %f usec(s)\n", float64(nativeElapsed.Microseconds()))
	} else {
		ulambdaElapsed := s.TestNinep()
		log.Printf("Ninep elapsed time: %f usec(s)\n", float64(ulambdaElapsed.Microseconds()))
	}
}
