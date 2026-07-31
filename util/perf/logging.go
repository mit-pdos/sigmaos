package perf

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

var (
	TIME_NOT_SET time.Time = time.Unix(0, 0)
)

// CPUNow returns the CPU time (user + system, across all this process's
// threads) consumed so far. Note that execve preserves these counters, so for
// a proc started by the uproc-trampoline the origin is the trampoline's fork,
// not main; take deltas rather than absolute values.
func CPUNow() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano()) + time.Duration(ru.Stime.Nano())
}

// CPUPhases partitions a proc's CPU time across the phases of its work, so
// that "where does this proc's CPU go" can be answered rather than guessed.
//
// Deliberately a chain of checkpoints rather than a set of nested timers:
// getrusage reports the whole process, so overlapping timers would
// double-count the CPU of concurrent goroutines. Consecutive Mark calls
// instead carve CPU into non-overlapping windows that sum to the proc's total,
// which makes the attribution checkable:
//
//	Setup.RuntimeInit.CPU + sum(<phase>.CPU) ~= Proc.exit.CPU
//
// The flip side is that a window is labeled by what the marking goroutine was
// doing, and CPU burned by *other* goroutines during that window lands in it
// too (e.g. the mapper's initOutput goroutine, which runs concurrently with
// its first splits). Read a window as "CPU spent while the proc was in this
// phase", not "CPU spent by this phase's code".
type CPUPhases struct {
	pid       sp.Tpid
	spawnTime time.Time
	lastCPU   time.Duration
	lastWall  time.Time
}

// NewCPUPhases returns a phase chain if the CPU_PHASE_BREAKDOWN perf selector
// is set for this proc, and nil otherwise — Mark is nil-safe, so a caller holds
// the result unconditionally and the marks become no-ops when it is off. Off is
// the default because the chain measures the setup path it also runs on.
func NewCPUPhases(pe *proc.ProcEnv) *CPUPhases {
	if !HasLabel(pe, CPU_PHASE_BREAKDOWN) {
		return nil
	}
	return &CPUPhases{
		pid:       pe.GetPID(),
		spawnTime: pe.GetSpawnTime(),
		lastCPU:   CPUNow(),
		lastWall:  time.Now(),
	}
}

// Mark closes the current window and opens the next one, logging (and
// returning) the CPU and wall time of the window that just ended. The CPU
// delta is logged as an op duration under "<name>.CPU", so it shows up
// alongside the wall-clock spawn-latency measurements in the same stats table.
//
// Safe to call on a nil *CPUPhases, so code paths that don't set one up (e.g.
// tests driving a Mapper directly) need no guards.
func (p *CPUPhases) Mark(name string) (cpu time.Duration, wall time.Duration) {
	if p == nil {
		return 0, 0
	}
	// The deltas are always computed, so the returned values are meaningful
	// whether or not logging is on: getrusage costs ~1us, against ~4.6us for
	// each log line it feeds, which is what the check below skips.
	nowCPU, nowWall := CPUNow(), time.Now()
	cpu, wall = nowCPU-p.lastCPU, nowWall.Sub(p.lastWall)
	p.lastCPU, p.lastWall = nowCPU, nowWall
	if !db.WillBePrinted(db.SPAWN_LAT) {
		return cpu, wall
	}
	LogSpawnLatency(name+".CPU", p.pid, p.spawnTime, nowWall.Add(-cpu))
	db.DPrintf(db.SPAWN_LAT, "[%s] %s.phase cpu:%v wall:%v", p.pid, name, cpu, wall)
	return cpu, wall
}

func LogRuntimeInitLatency(pe *proc.ProcEnv) {
	pid, spawnTime := pe.GetPID(), pe.GetSpawnTime()
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
	// The wall-clock exec -> main latency is a plain spawn-latency metric, so
	// it is reported whenever spawn latencies are; only the CPU breakdown of
	// that window is gated.
	LogSpawnLatency("Setup.RuntimeInit", pid, spawnTime, execTime)
	if HasLabel(pe, CPU_PHASE_BREAKDOWN) {
		logRuntimeInitRusage(pid, spawnTime)
	}
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

// LogProcExitRusage reports the CPU and memory a proc consumed over its whole
// lifetime, logged as it exits. Read it against Setup.RuntimeInit.CPU to see
// what fraction of a proc's CPU went into getting to main rather than into its
// work — the thing that decides how much shrinking the setup path can buy. As
// with Setup.RuntimeInit.rusage, the counters include the trampoline's share
// (reported separately as trampCPU there), since execve preserves them.
func LogProcExitRusage(pe *proc.ProcEnv) {
	if !HasLabel(pe, CPU_PHASE_BREAKDOWN) || !db.WillBePrinted(db.SPAWN_LAT) {
		return
	}
	pid, spawnTime := pe.GetPID(), pe.GetSpawnTime()
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		db.DPrintf(db.SPAWN_LAT, "[%s] Proc.exit.rusage err %v", pid, err)
		return
	}
	utime := time.Duration(ru.Utime.Nano())
	stime := time.Duration(ru.Stime.Nano())
	// Logged as an op duration too, so it shows up in the spawn-latency stats
	// next to Setup.RuntimeInit.CPU.
	LogSpawnLatency("Proc.exit.CPU", pid, spawnTime, time.Now().Add(-(utime + stime)))
	db.DPrintf(db.SPAWN_LAT, "[%s] Proc.exit.rusage cpu:%v utime:%v stime:%v maxrssKB:%d minflt:%d majflt:%d nvcsw:%d nivcsw:%d", pid, utime+stime, utime, stime, ru.Maxrss, ru.Minflt, ru.Majflt, ru.Nvcsw, ru.Nivcsw)
}

// LogCPUSince logs the CPU consumed since startCPU (from CPUNow) as
// "<name>.CPU". Gated on CPU_PHASE_BREAKDOWN, like the rest of the CPU
// accounting; pair it with CPUSinceStart so the getrusage at the other end is
// skipped too.
//
// Unlike CPUPhases.Mark this partitions nothing: use it for work that runs
// concurrently with the phases, whose CPU is *also* counted in whichever
// window it overlapped. Reported separately precisely because it can't be
// added to the chain without double-counting.
func LogCPUSince(name string, pe *proc.ProcEnv, startCPU time.Duration) {
	if !HasLabel(pe, CPU_PHASE_BREAKDOWN) || !db.WillBePrinted(db.SPAWN_LAT) {
		return
	}
	cpu := CPUNow() - startCPU
	LogSpawnLatency(name+".CPU", pe.GetPID(), pe.GetSpawnTime(), time.Now().Add(-cpu))
}

// CPUStart returns the CPU reading LogCPUSince expects, or 0 when the CPU
// breakdown is off, so the caller pays no getrusage either way.
func CPUStart(pe *proc.ProcEnv) time.Duration {
	if !HasLabel(pe, CPU_PHASE_BREAKDOWN) {
		return 0
	}
	return CPUNow()
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
