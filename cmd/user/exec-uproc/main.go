package main

import (
	"os"
	"strconv"
	"time"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/proc"
)

func main() {
	execTimeStr := os.Getenv("SIGMA_EXEC_TIME")
	execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
	if err != nil {
		db.DFatalf("Error parsing exec time: %v", err)
	}
	execTime := time.UnixMicro(execTimeMicro)
	db.DPrintf(db.SPAWN_LAT, "[%v] Trampoline exec latency: %v", proc.GetSigmaDebugPid(), time.Since(execTime))
	if err := container.ExecUProc(); err != nil {
		db.DFatalf("%v: ExecUProc err %v\n", os.Args[0], err)
	}
	os.Exit(0)
}
