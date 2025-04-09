package main

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/util/perf"
)

func main() {
	if os.Getenv("IS_FORKTEST") == "true" {
		if len(os.Args) == 1 {
			db.DPrintf(db.ALWAYS, "In parent")
			cmd := exec.Command("./bin", []string{strconv.FormatInt(time.Now().UnixMicro(), 10)}...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				db.DFatalf("Error start %v %v", cmd, err)
			}
			if err := cmd.Wait(); err != nil {
				db.DFatalf("Error wait %v %v", cmd, err)
			}
			return
		} else {
			execTimeStr := os.Args[1]
			execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
			if err != nil {
				db.DFatalf("Error parsing exec time 1: %v", err)
			}
			execTime := time.UnixMicro(execTimeMicro)
			db.DPrintf(db.ALWAYS, "Trampoline exec latency: %v", time.Since(execTime))
			db.DPrintf(db.ALWAYS, "In child")
			return
		}
	}

	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}

	execTimeStr := os.Getenv("SIGMA_EXEC_TIME")
	execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
	if err != nil {
		db.DFatalf("Error parsing exec time 2: %v", err)
	}
	pe := proc.GetProcEnv()
	execTime := time.UnixMicro(execTimeMicro)
	execLat := time.Since(execTime)
	perf.LogSpawnLatency("SpawnBench.Exec", pe.GetPID(), pe.GetSpawnTime(), execTime)
	spawnLat := time.Since(pe.GetSpawnTime())
	l, err := NewSpawnBench(pe, execLat, spawnLat)
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	l.Work()
}

type SpawnBench struct {
	execLat  time.Duration
	spawnLat time.Duration
	*sigmaclnt.SigmaClnt
}

func NewSpawnBench(pe *proc.ProcEnv, execLat time.Duration, spawnLat time.Duration) (*SpawnBench, error) {
	s := &SpawnBench{
		execLat:  execLat,
		spawnLat: spawnLat,
	}
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("NewSigmaClient: %v", err)
	}
	s.SigmaClnt = sc
	return s, nil
}

func (s *SpawnBench) Work() {
	start := time.Now()
	err := s.Started()
	if err != nil {
		db.DFatalf("Started error: %v", err)
	}
	start = time.Now()
	s.ClntExit(proc.NewStatusInfo(proc.StatusOK, "Spawn latency until main", s.spawnLat))
	perf.LogSpawnLatency("SpawnBench.Exited", s.ProcEnv().GetPID(), s.ProcEnv().GetSpawnTime(), execTime)
}
