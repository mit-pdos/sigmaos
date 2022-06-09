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
func (pd *Procd) allocCores(n proc.Tcore) []uint {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	cores := []uint{}
	for i := 0; i < len(pd.coreBitmap); i++ {
		coreStatus := pd.coreBitmap[i]
		// If this core is not assigned to this procd, move on.
		if coreStatus == CORE_BLOCKED {
			continue
		}
		// If lambda asks for 0 cores, run on any unblocked core.
		if n == proc.C_DEF {
			cores = append(cores, uint(i))
			continue
		}
		// If core is idle, claim it.
		if coreStatus == CORE_IDLE {
			pd.coreBitmap[i] = CORE_BUSY
			cores = append(cores, uint(i))
			n -= 1
			if n == 0 {
				break
			}
		}
	}
	return cores
}

// Free a set of cores which was being used by a proc.
func (pd *Procd) freeCores(ncore proc.Tcore, cores []uint) {
	// If no cores were exclusively allocated to this proc, return immediately.
	if ncore == proc.C_DEF {
		return
	}

	pd.mu.Lock()
	defer pd.mu.Unlock()

	for _, i := range cores {
		if pd.coreBitmap[i] == CORE_IDLE {
			debug.PrintStack()
			db.DFatalf("Error: Double free cores")
		}
		pd.coreBitmap[i] = CORE_IDLE
	}
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
