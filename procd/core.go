package procd

import (
	"runtime/debug"

	db "ulambda/debug"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/resource"
)

type Tcorestatus uint8

const (
	CORE_IDLE    Tcorestatus = iota
	CORE_BUSY                // Currently occupied by a proc
	CORE_BLOCKED             // Not for use by this procd's procs.
)

func (pd *Procd) addCores(msg *resource.ResourceMsg) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Increment the count of available cores.
	pd.coresAvail += proc.Tcore(msg.Amount)
	// Sanity check
	if pd.coresAvail > proc.Tcore(linuxsched.NCores) {
		db.DFatalf("Added more procd cores than there are on this machine: %v > %v", pd.coresAvail, linuxsched.NCores)
	}
	cores := parseCoreSlice(msg)
	pd.markCoresL(cores, CORE_IDLE)
	// TODO: rebalance work across new cores.
	// Notify sleeping workers that they may be able to run procs now.
	go func() {
		for i := 0; i < msg.Amount; i++ {
			pd.stealChan <- true
		}
	}()
}

func (pd *Procd) removeCores(msg *resource.ResourceMsg) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Sanity check
	cores := parseCoreSlice(msg)
	pd.coresAvail -= proc.Tcore(msg.Amount)
	for _, i := range cores {
		// Cores which were busy will already have been subtracted from the number
		// of available cores. Avoid double-subtracting them here.
		if pd.coreBitmap[i] == CORE_BUSY {
			pd.coresAvail += 1
		}
	}
	if pd.coresAvail < proc.Tcore(0) {
		db.DFatalf("Added more procd cores than there are on this machine: %v > %v", pd.coresAvail, linuxsched.NCores)
	}
	pd.markCoresL(cores, CORE_BLOCKED)
	// TODO: rebalance work across new cores, evict some procs, etc.
}

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
func (pd *Procd) allocCores(p *LinuxProc) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	allocated := proc.Tcore(0)
	// XXX set don't append.
	p.cores = []uint{}
	for i := 0; i < len(pd.coreBitmap); i++ {
		coreStatus := pd.coreBitmap[i]
		// If this core is not assigned to this procd, move on.
		if coreStatus == CORE_BLOCKED {
			continue
		}
		// If lambda asks for 0 cores, run on any unblocked core.
		if p.attr.Ncore == proc.C_DEF {
			p.cores = append(p.cores, uint(i))
			continue
		}
		// If core is idle, claim it.
		if coreStatus == CORE_IDLE {
			p.cores = append(p.cores, uint(i))
			allocated += 1
			if allocated == p.attr.Ncore {
				break
			}
		}
	}

	// Mark cores as busy, if this proc asked for exclusive access to cores.
	if p.attr.Ncore > 0 {
		pd.markCoresL(p.cores, CORE_BUSY)
	}
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
func (pd *Procd) freeCores(p *LinuxProc) {
	// If no cores were exclusively allocated to this proc, return immediately.
	if p.attr.Ncore == proc.C_DEF {
		return
	}

	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.markCoresL(p.cores, CORE_IDLE)
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

func parseCoreSlice(msg *resource.ResourceMsg) []uint {
	iv := np.MkInterval(0, 0)
	iv.Unmarshal(msg.Name)
	cores := make([]uint, msg.Amount)
	for i := uint(0); i < uint(msg.Amount); i++ {
		cores[i] = uint(iv.Start) + i
	}
	return cores
}
