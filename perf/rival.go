package perf

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"ulambda/fslib"
)

const (
	MONITOR_HZ = 10
)

type Rival struct {
	spawnsPerSec       int
	secs               float64
	sleepIntervalUsecs int
	killed             bool
	ninep              bool
	*fslib.FsLib
}

func MakeRival(args []string) (*Rival, error) {
	if len(args) < 4 {
		return nil, errors.New("MakeRival: too few arguments")
	}
	log.Printf("MakeRival: %v\n", args)

	r := &Rival{}
	r.FsLib = fslib.MakeFsLib("rival")

	sps, err := strconv.Atoi(args[0])
	r.spawnsPerSec = sps
	if err != nil {
		log.Fatalf("Invalid num spawns per sec: %v, %v\n", args[0], err)
	}

	secs, err := strconv.Atoi(args[1])
	r.secs = float64(secs)
	if err != nil {
		log.Fatalf("Invalid num seconds: %v, %v\n", args[0], err)
	}

	r.sleepIntervalUsecs = 1000000 / r.spawnsPerSec
	if r.secs >= 0 {
		log.Printf("Spawning every %v usec(s) for %v secs", r.sleepIntervalUsecs, r.secs)
	} else {
		log.Printf("Spawning every %v usec(s) indefinitely", r.sleepIntervalUsecs)
	}

	if args[2] == "native" {
		r.ninep = false
	} else if args[2] == "ninep" {
		r.ninep = true
	} else {
		log.Fatalf("Unexpected rival spawn type: %v", args[2])
	}

	return r, nil
}

func (r *Rival) spawnSpinner(pid string) {
	dim := "64"
	its := "60"
	if r.ninep {
		a := &fslib.Attr{pid, "bin/c-spinner", "", []string{dim, its}, nil, nil, nil, 0}
		err := r.Spawn(a)
		if err != nil {
			log.Fatalf("couldn't spawn ninep spinner %v: %v\n", pid, err)
		}
	} else {
		cmd := exec.Command("./bin/c-spinner", []string{pid, dim, its, "native"}...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Start()
		if err != nil {
			log.Printf("Error starting native spinner: %v, %v\n", pid, err)
		}
	}
}

func (r *Rival) Work() {
	start := time.Now()
	for {
		// Check if we're done
		if r.secs >= 0 && time.Now().Sub(start).Seconds() >= r.secs {
			break
		}
		pid := fslib.GenPid()
		r.spawnSpinner(pid)
		time.Sleep(time.Duration(r.sleepIntervalUsecs) * time.Microsecond)
	}
	r.killed = true
}
