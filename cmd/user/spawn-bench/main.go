package main

import (
	"os"
	"strconv"
	"time"

	"os/exec"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
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
	execTime := time.UnixMicro(execTimeMicro)
	execLat := time.Since(execTime)
	db.DPrintf(db.SPAWN_LAT, "[%v] Proc exec latency: %v", proc.GetSigmaDebugPid(), execLat)
	pe := proc.GetProcEnv()
	spawnLat := time.Since(pe.GetSpawnTime())
	db.DPrintf(db.SPAWN_LAT, "[%v] E2e latency until main: %v", pe.GetPID(), spawnLat)
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
	s.ClntExit(proc.NewStatusInfo(proc.StatusOK, "Spawn latency until main", s.spawnLat))
}
