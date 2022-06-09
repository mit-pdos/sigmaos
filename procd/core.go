package procd

import (
	"runtime/debug"

	db "ulambda/debug"
	"ulambda/proc"
)

type Tcorestatus uint8

const (
	CORE_IDLE    Tcorestatus = iota
	CORE_BUSY                // Currently occupied by a proc
	CORE_BLOCKED             // Not for use by this procd's procs.
)

// XXX Statsd information?
// Check if this procd has enough cores to run proc p. Caller holds lock.
func (pd *Procd) hasEnoughCores(p *proc.Proc) bool {
	// If we have enough cores to run this job...
	if pd.coresAvail >= p.Ncore {
		return true
	}
	return false
}

// Allocate n cores to a proc, and note it occupies in the core bitmap.
func (pd *Procd) allocCores(p *proc.Proc) []uint {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	allocated := proc.Tcore(0)
	cores := []uint{}
	for i := 0; i < len(pd.coreBitmap); i++ {
		coreStatus := pd.coreBitmap[i]
		// If this core is not assigned to this procd, move on.
		if coreStatus == CORE_BLOCKED {
			continue
		}
		// If lambda asks for 0 cores, run on any unblocked core.
		if p.Ncore == proc.C_DEF {
			cores = append(cores, uint(i))
			continue
		}
		// If core is idle, claim it.
		if coreStatus == CORE_IDLE {
			cores = append(cores, uint(i))
			allocated += 1
			if allocated == p.Ncore {
				break
			}
		}
	}

	// Mark cores as busy, if this proc asked for exclusive access to cores.
	if p.Ncore > 0 {
		pd.markCoresL(cores, CORE_BUSY)
	}

	return cores
}

// Set the status of a set of cores. Caller holds lock.
func (pd *Procd) markCoresL(cores []uint, status Tcorestatus) {
	for _, i := range cores {
		// If we are double-setting a core's status, it's probably a bug.
		if pd.coreBitmap[i] == status {
			debug.PrintStack()
			db.DFatalf("Error: Double-marked cores %v", status)
		}
		pd.coreBitmap[i] = status
	}
}

// Free a set of cores which was being used by a proc.
func (pd *Procd) freeCores(p *proc.Proc, cores []uint) {
	// If no cores were exclusively allocated to this proc, return immediately.
	if p.Ncore == proc.C_DEF {
		return
	}

	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.markCoresL(cores, CORE_IDLE)
}

// Update resource accounting information. Caller holds lock.
func (pd *Procd) decrementCoresL(p *proc.Proc) {
	pd.coresAvail -= p.Ncore
}

// Update resource accounting information.
func (pd *Procd) incrementCores(p *proc.Proc) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.coresAvail += p.Ncore
}
