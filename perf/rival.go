package perf

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
	if len(args) < 3 {
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

func getCPUSample() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					log.Printf("Error: %v %v %v", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}

func (r *Rival) monitorCpuUtil() {
	sleepMsecs := 1000 / MONITOR_HZ
	var idle0 uint64
	var total0 uint64
	var idle1 uint64
	var total1 uint64
	idle0, total0 = getCPUSample()
	for !r.killed {
		time.Sleep(time.Duration(sleepMsecs) * time.Millisecond)
		idle1, total1 = getCPUSample()
		idleDelta := float64(idle1 - idle0)
		totalDelta := float64(total1 - total0)
		util := 100.0 * (totalDelta - idleDelta) / totalDelta
		log.Printf("CPU util: %f [busy: %f, total: %f]\n", util, totalDelta-idleDelta, totalDelta)
		idle0 = idle1
		total0 = total1
	}
}

func (r *Rival) Work() {
	go r.monitorCpuUtil()
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
