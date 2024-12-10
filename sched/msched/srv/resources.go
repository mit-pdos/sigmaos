package srv

import (
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/util/linux/mem"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

type cpuStats struct {
	idle  uint64
	total uint64
}

func (msched *MSched) getCPUUtil() int64 {
	return atomic.LoadInt64(&msched.cpuUtil)
}

func (msched *MSched) monitorCPU() {
	cm := perf.GetActiveCores()
	t := time.NewTicker(sp.Conf.MSched.UTIL_REFRESH_RATE)
	var oldStats cpuStats
	for {
		<-t.C
		oldStats = *msched.cpuStats
		idle, total := perf.GetCPUSample(cm)
		msched.cpuStats.idle = idle
		msched.cpuStats.total = total
		if oldStats.idle == 0 && oldStats.total == 0 {
			continue
		}
		idleDelta := float64(msched.cpuStats.idle - oldStats.idle)
		totalDelta := float64(msched.cpuStats.total - oldStats.total)
		utilPct := 100.0 * (totalDelta - idleDelta) / totalDelta
		atomic.StoreInt64(&msched.cpuUtil, int64(utilPct))
	}
}

func (msched *MSched) allocMem(m proc.Tmem) {
	msched.mu.Lock()
	defer msched.mu.Unlock()

	// Sanity checks
	if msched.memfree < m {
		db.DFatalf("Error alloc more mem than available: %v > %v", m, msched.memfree)
	}
	msched.memfree -= m
	// Sanity checks
	if msched.memfree > mem.GetTotalMem() {
		db.DFatalf("Error mem overflow: %v > %v", msched.memfree, mem.GetTotalMem())
	}
}

func (msched *MSched) getFreeMem() proc.Tmem {
	msched.mu.Lock()
	defer msched.mu.Unlock()

	return msched.memfree
}

func (msched *MSched) freeMem(m proc.Tmem) {
	msched.mu.Lock()
	defer msched.mu.Unlock()

	msched.memfree += m
	// Sanity checks
	if msched.memfree > mem.GetTotalMem() {
		db.DFatalf("Error mem overflow: %v > %v", msched.memfree, mem.GetTotalMem())
	}
	// Signal that a new proc may be runnable.
	msched.cond.Signal()
}

func (msched *MSched) waitForMoreMem() {
	msched.mu.Lock()
	defer msched.mu.Unlock()

	// Wait until there is free memory
	for msched.memfree == 0 {
		msched.cond.Wait()
	}
}
