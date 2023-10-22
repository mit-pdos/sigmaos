package schedd

import (
	db "sigmaos/debug"
	"sigmaos/mem"
	"sigmaos/proc"
)

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
