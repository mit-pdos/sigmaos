package schedsrv

import (
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/mem"
	"sigmaos/util/perf"
	"sigmaos/proc"
)

const (
	TARGET_CPU_PCT    = 95
	UTIL_REFRESH_RATE = 20 * time.Millisecond
)

type cpuStats struct {
	idle  uint64
	total uint64
}

func (sd *Schedd) getCPUUtil() int64 {
	return atomic.LoadInt64(&sd.cpuUtil)
}

func (sd *Schedd) monitorCPU() {
	cm := perf.GetActiveCores()
	t := time.NewTicker(UTIL_REFRESH_RATE)
	var oldStats cpuStats
	for {
		<-t.C
		oldStats = *sd.cpuStats
		idle, total := perf.GetCPUSample(cm)
		sd.cpuStats.idle = idle
		sd.cpuStats.total = total
		if oldStats.idle == 0 && oldStats.total == 0 {
			continue
		}
		idleDelta := float64(sd.cpuStats.idle - oldStats.idle)
		totalDelta := float64(sd.cpuStats.total - oldStats.total)
		utilPct := 100.0 * (totalDelta - idleDelta) / totalDelta
		atomic.StoreInt64(&sd.cpuUtil, int64(utilPct))
	}
}

func (sd *Schedd) allocMem(m proc.Tmem) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Sanity checks
	if sd.memfree < m {
		db.DFatalf("Error alloc more mem than available: %v > %v", m, sd.memfree)
	}
	sd.memfree -= m
	// Sanity checks
	if sd.memfree > mem.GetTotalMem() {
		db.DFatalf("Error mem overflow: %v > %v", sd.memfree, mem.GetTotalMem())
	}
}

func (sd *Schedd) getFreeMem() proc.Tmem {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	return sd.memfree
}

func (sd *Schedd) freeMem(m proc.Tmem) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.memfree += m
	// Sanity checks
	if sd.memfree > mem.GetTotalMem() {
		db.DFatalf("Error mem overflow: %v > %v", sd.memfree, mem.GetTotalMem())
	}
	// Signal that a new proc may be runnable.
	sd.cond.Signal()
}

func (sd *Schedd) waitForMoreMem() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Wait until there is free memory
	for sd.memfree == 0 {
		sd.cond.Wait()
	}
}
