package perf

import (
	"log"
	//	"sync"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/debug"
	"ulambda/fslib"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *fslib.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	bin := ".."
	s, err := fslib.Boot(bin)
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.s = s

	ts.FsLib = fslib.MakeFsLib("spinner")
	ts.t = t

	return ts
}

func makeTstateNoBoot(t *testing.T, s *fslib.System, pid string) *Tstate {
	ts := &Tstate{}
	ts.FsLib = fslib.MakeFsLib("spinner_" + pid)
	ts.t = t
	ts.s = s
	return ts
}

func spawnSpinnerWithPid(t *testing.T, ts *Tstate, pid string, msecs string) {
	a := &fslib.Attr{pid, "bin/spinner", "", []string{msecs}, nil, nil, nil}
	err := ts.Spawn(a)
	assert.Nil(t, err, "Spawn")
}

func spawnSpinner(t *testing.T, ts *Tstate, msecs string) string {
	pid := fslib.GenPid()
	spawnSpinnerWithPid(t, ts, pid, msecs)
	return pid
}

func spawnNoOp(t *testing.T, ts *Tstate, deps []string) string {
	pid := fslib.GenPid()
	err := ts.SpawnNoOp(pid, deps)
	assert.Nil(t, err, "SpawnNoOp")

	log.Printf("SpawnNoOp %v\n", pid)
	return pid
}

func TestSpinners(t *testing.T) {
	ts := makeTstate(t)

	// Test params
	spinMsecs := 2000
	nSpinners := 100

	msecs := strconv.Itoa(spinMsecs)
	pids := map[string]int{}

	// Gen pids
	for i := 0; i < nSpinners; i++ {
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
		spawnSpinnerWithPid(t, ts, pid, msecs)
	}

	// Wait for them all
	for pid, _ := range pids {
		ts.Wait(pid)
	}
	end := time.Now()

	// Calculate elapsed time
	elapsed := end.Sub(start)
	log.Printf("Elapsed time: %v msec(s)\n", elapsed.Milliseconds())

	ts.s.Shutdown(ts.FsLib)
}
