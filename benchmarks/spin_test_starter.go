package benchmarks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

type SpinTestStarter struct {
	nSpinners int
	dim       string
	its       string
	native    bool
	baseline  bool
	aws       bool
	local     bool
	perfStat  bool
	*fslib.FsLib
	*procclnt.ProcClnt
}

func (s *SpinTestStarter) spawnSpinnerWithPid(pid proc.Tpid) {
	var a *proc.Proc
	if s.perfStat {
		a = proc.MakeProcPid(pid, "user/perf", []string{"stat", ".user/c-spinner", pid.String(), s.dim, s.its})
	} else {
		a = proc.MakeProcPid(pid, "user/c-spinner", []string{s.dim, s.its})
	}
	err := s.Spawn(a)
	if err != nil {
		db.DFatalf("couldn't spawn %v: %v\n", pid, err)
	}
}

func (s *SpinTestStarter) spawnSpinner() proc.Tpid {
	pid := proc.GenPid()
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
		db.DFatalf("Can't run as local & aws\n")
	}

	if !s.native {
		s.FsLib = fslib.MakeFsLib("spin-test-starter")
		s.ProcClnt = procclnt.MakeProcClnt(s.FsLib)
	}

	nSpinners, err := strconv.Atoi(args[0])
	s.nSpinners = nSpinners
	if err != nil {
		db.DFatalf("Invalid dimension: %v, %v\n", args[0], err)
	}

	_, err = strconv.Atoi(args[1])
	s.dim = args[1]
	if err != nil {
		db.DFatalf("Invalid dimension: %v, %v\n", args[1], err)
	}

	i, err := strconv.Atoi(args[2])
	if err != nil {
		db.DFatalf("Invalid num interations: %v, %v\n", args[2], err)
	}

	// Limited by the AWS API Gateway timeout
	if i > 5000 && s.local == false {
		s.its = "5000"
	} else {
		s.its = args[2]
	}

	if len(args) == 6 && args[5] == "perf_stat" {
		s.perfStat = true
	}

	return s, nil
}

func (s *SpinTestStarter) TestNinep() time.Duration {
	pids := map[proc.Tpid]int{}

	// Gen pids
	for i := 0; i < s.nSpinners; i++ {
		p := proc.GenPid()
		_, alreadySpawned := pids[p]
		for alreadySpawned {
			p = proc.GenPid()
			_, alreadySpawned = pids[p]
		}
		pids[p] = i
	}

	start := time.Now()
	// Start some lambdas
	for pid, _ := range pids {
		s.spawnSpinnerWithPid(pid)
	}

	// Wait for them all
	for pid, _ := range pids {
		s.WaitExit(pid)
	}
	end := time.Now()

	// Print out the results from perf stat
	if s.perfStat {
		for pid, _ := range pids {
			fname := "/tmp/perf-stat-" + pid.String() + ".out"
			contents, err := ioutil.ReadFile(fname)
			if err != nil {
				log.Printf("Couldn't read file contents: %v, %v", fname, err)
			}
			os.Remove(fname)
			log.Printf("%v", string(contents))
		}
	}

	// Calculate elapsed time
	elapsed := end.Sub(start)
	return elapsed
}

func (s *SpinTestStarter) TestNative() time.Duration {
	pids := map[proc.Tpid]*exec.Cmd{}

	// Gen pids
	for i := 0; i < s.nSpinners; i++ {
		p := proc.GenPid()
		pid := p.String() + "-native-" + s.its
		_, alreadySpawned := pids[p]
		for alreadySpawned {
			p = proc.GenPid()
			_, alreadySpawned = pids[p]
		}

		// Set up command struct
		var cmd *exec.Cmd
		if s.perfStat {
			cmd = exec.Command("perf", []string{"stat", ".user/c-spinner", pid, s.dim, s.its, "native"}...)
			fname := "/tmp/perf-stat-" + pid + ".out"
			file, err := os.Create(fname)
			if err != nil {
				db.DFatalf("Error creating perf stat output file: %v, %v", fname, err)
			}
			cmd.Stdout = file
			cmd.Stderr = file
		} else {
			cmd = exec.Command(".user/c-spinner", []string{pid, s.dim, s.its, "native"}...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		// Store it along
		pids[p] = cmd
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

	if s.perfStat {
		// Print out the results from perf stat
		for pid, _ := range pids {
			fname := "/tmp/perf-stat-" + pid.String() + ".out"
			contents, err := ioutil.ReadFile(fname)
			if err != nil {
				log.Printf("Couldn't read file contents: %v, %v", fname, err)
			}
			os.Remove(fname)
			log.Printf("%v", string(contents))
		}
	}

	// Calculate elapsed time
	elapsed := end.Sub(start)
	return elapsed
}

func (s *SpinTestStarter) TestAws() time.Duration {
	vals := map[string]bool{"baseline": true}
	body, err := json.Marshal(vals)

	if err != nil {
		db.DFatalf("Error marshalling for lamda baseline: %v", err)
	}

	url := fmt.Sprintf("https://m5ica91644.execute-api.us-east-1.amazonaws.com/default/cpp-spin?dim=%v&its=%v", s.dim, s.its)

	start := time.Now()
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	end := time.Now()

	if resp.StatusCode != 200 {
		db.DFatalf("Bad response code: %v, %v", resp.StatusCode, resp)
	}

	if err != nil {
		db.DFatalf("Error in HTTP POST: %v", err)
	}

	// Calculate elapsed time
	elapsed := end.Sub(start)
	return elapsed
}

func (s *SpinTestStarter) TestLocalBaseline() {
	pid := proc.GenPid()

	// Set up command struct
	cmd := exec.Command(".user/c-spinner", []string{pid.String(), s.dim, s.its, "baseline"}...)
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
		db.DFatalf("Error marshalling for lamda baseline: %v", err)
	}

	url := fmt.Sprintf("https://m5ica91644.execute-api.us-east-1.amazonaws.com/default/cpp-spin?dim=%v&its=%v", s.dim, s.its)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))

	if resp.StatusCode != 200 {
		db.DFatalf("Bad response code: %v, %v", resp.StatusCode, resp)
	}

	if err != nil {
		db.DFatalf("Error in HTTP POST: %v", err)
	}

	var res map[string]interface{}

	json.NewDecoder(resp.Body).Decode(&res)

	if err != nil {
		db.DFatalf("Error unmarshalling response body: %v", err)
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
