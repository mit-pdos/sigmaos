package perf

import (
	"errors"
	"log"
	"strconv"
	"time"

	"ulambda/fslib"
)

type Rival struct {
	spawnsPerSec       int
	secs               float64
	sleepIntervalMsecs int
	*fslib.FsLib
}

func MakeRival(args []string) (*Rival, error) {
	if len(args) < 2 {
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

	r.sleepIntervalMsecs = 1000 / r.spawnsPerSec
	log.Printf("Spawning every %v msec", r.sleepIntervalMsecs)

	return r, nil
}

func (r *Rival) Work() {
	start := time.Now()
	for {
		// Check if we're done
		if time.Now().Sub(start).Seconds() >= r.secs {
			break
		}
		pid := fslib.GenPid()
		r.spawnSpinnerWithPid(pid)
		time.Sleep(time.Duration(r.sleepIntervalMsecs) * time.Millisecond)
	}
}

func (r *Rival) spawnSpinnerWithPid(pid string) {
	dim := "64"
	its := "60"
	a := &fslib.Attr{pid, "bin/c-spinner", "", []string{dim, its}, nil, nil, nil, 0}
	err := r.Spawn(a)
	if err != nil {
		log.Fatalf("couldn't spawn %v: %v\n", pid, err)
	}
}
