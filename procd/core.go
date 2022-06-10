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
	// Notify sleeping workers that they may be able to run procs now.
	go func() {
		for i := 0; i < msg.Amount; i++ {
			pd.stealChan <- true
		}
	}()
	pd.rebalanceProcs()
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
	pd.rebalanceProcs()
}

// Rebalances procs across set of available cores.
func (pd *Procd) rebalanceProcs() {
	// TODO
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

// Allocate cores to a proc, and assign cores to it in the core bitmap. Caller
// holds lock.
func (pd *Procd) allocCoresL(p *LinuxProc) {
	// Number of cores allocated so ar.
	allocated := 0
	for i := 0; i < len(pd.coreBitmap) && allocated < len(p.cores); i++ {
		// If we have allocated the right number of cores already, break.
		coreStatus := pd.coreBitmap[i]
		// If this core is not assigned to this procd, move on.
		if coreStatus == CORE_BLOCKED {
			continue
		}
		// If lambda asks for 0 cores, or the core is idle, then the proc can run
		// on this core. Claim it.
		if p.attr.Ncore == proc.C_DEF || coreStatus == CORE_IDLE {
			p.cores[allocated] = uint(i)
			allocated++
		}
	}

	// Mark cores as busy, if this proc asked for exclusive access to cores.
	if p.attr.Ncore > 0 {
		pd.coresAvail -= proc.Tcore(len(p.cores))
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
	pd.coresAvail += proc.Tcore(len(p.cores))
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
