package perf

import (
	"fmt"
	"os"
	"strconv"
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

var (
	TIME_NOT_SET time.Time = time.Unix(0, 0)
)

func LogRuntimeInitLatency(pid sp.Tpid, spawnTime time.Time) {
	execTimeStr := os.Getenv("SIGMA_EXEC_TIME")
	execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error parsing exec time 2: %v", err)
		return
	}
	execTime := time.UnixMicro(execTimeMicro)
	LogSpawnLatency("Initialization.RuntimeInit", pid, spawnTime, execTime)
}

// Some convenience functions for logging performance-related data
func LogSpawnLatency(format string, pid sp.Tpid, spawnTime time.Time, opStart time.Time, v ...interface{}) {
	// Bail out early if not logging
	if !db.WillBePrinted(db.SPAWN_LAT) {
		return
	}
	var sinceSpawn time.Duration
	if spawnTime != TIME_NOT_SET {
		sinceSpawn = time.Since(spawnTime)
	}
	var sinceOpStart time.Duration
	if opStart != TIME_NOT_SET {
		sinceOpStart = time.Since(opStart)
	}
	db.DPrintf(db.SPAWN_LAT, "[%s] %s op:%v sinceSpawn:%v", pid, fmt.Sprintf(format, v...), sinceOpStart, sinceSpawn)
}

// Some convenience functions for logging performance-related data
func LogSpawnLatencyVerbose(format string, pid sp.Tpid, spawnTime time.Time, opStart time.Time, v ...interface{}) {
	// Bail out early if not logging
	if !db.WillBePrinted(db.SPAWN_LAT_VERBOSE) {
		return
	}
	var sinceSpawn time.Duration
	if spawnTime != TIME_NOT_SET {
		sinceSpawn = time.Since(spawnTime)
	}
	var sinceOpStart time.Duration
	if opStart != TIME_NOT_SET {
		sinceOpStart = time.Since(opStart)
	}
	db.DPrintf(db.SPAWN_LAT_VERBOSE, "[%s] %s op:%v sinceSpawn:%v", pid, fmt.Sprintf(format, v...), sinceOpStart, sinceSpawn)
}
