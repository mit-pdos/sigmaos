package perf

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

var (
	TIME_NOT_SET time.Time = time.Unix(0, 0)
)

func LogRuntimeInitLatency(pid sp.Tpid, spawnTime time.Time) {
	execTimeStr := os.Getenv("SIGMA_EXEC_TIME")
	// If not set, bail out
	if execTimeStr == "" {
		return
	}
	execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
	if err != nil {
		db.DFatalf("Error parsing exec time 2: %v", err)
		return
	}
	execTime := time.UnixMicro(execTimeMicro)
	LogSpawnLatency("Setup.RuntimeInit", pid, spawnTime, execTime)
	logRuntimeInitRusage(pid, spawnTime)
}

// logRuntimeInitRusage reports how much of the exec -> main window was work
// this process actually did, versus time it spent waiting for a CPU.
//
// execve keeps the process's CPU counters (only fork resets them), so
// getrusage(RUSAGE_SELF) here also covers the trampoline's work before it
// exec'd us. The trampoline hands over its own CPU time in SIGMA_EXEC_CPU_US
// so we can subtract it and report the cost of exec -> main alone: ELF load,
// dynamic linking, Go runtime init and package inits — the same window as
// Setup.RuntimeInit. Fault and context-switch counts are reported raw (they
// include the trampoline, whose share is small and visible in
// Setup.Isolation). Compare CPU against wall:
//
//   - Setup.RuntimeInit.CPU (utime+stime) flat while Setup.RuntimeInit grows
//     => the work didn't change, the process was waiting for a CPU
//     (oversubscription). nivcsw (involuntary context switches) corroborates.
//   - both grow => the work itself got more expensive (e.g. more faults served
//     through binfs, cache/TLB pressure).
//
// majflt counts faults that needed I/O — for a binary exec'd from binfs, those
// are FUSE round trips into procd; minflt counts the cheap page-cache hits.
func logRuntimeInitRusage(pid sp.Tpid, spawnTime time.Time) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		db.DPrintf(db.SPAWN_LAT, "[%s] Setup.RuntimeInit.rusage err %v", pid, err)
		return
	}
	utime := time.Duration(ru.Utime.Nano())
	stime := time.Duration(ru.Stime.Nano())
	cpu := utime + stime
	// Subtract the CPU the trampoline burned before exec'ing us, if it told us.
	var tramp time.Duration
	if s := os.Getenv("SIGMA_EXEC_CPU_US"); s != "" {
		if us, err := strconv.ParseInt(s, 10, 64); err == nil {
			tramp = time.Duration(us) * time.Microsecond
			if tramp <= cpu {
				cpu -= tramp
			}
		}
	}
	// Logged as an op duration so that it shows up alongside
	// Setup.RuntimeInit in the spawn-latency stats.
	LogSpawnLatency("Setup.RuntimeInit.CPU", pid, spawnTime, time.Now().Add(-cpu))
	db.DPrintf(db.SPAWN_LAT, "[%s] Setup.RuntimeInit.rusage cpu:%v utime:%v stime:%v trampCPU:%v minflt:%d majflt:%d nvcsw:%d nivcsw:%d", pid, cpu, utime, stime, tramp, ru.Minflt, ru.Majflt, ru.Nvcsw, ru.Nivcsw)
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
