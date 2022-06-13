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
	cores := parseCoreInterval(msg)
	pd.adjustCoresOwned(pd.coresOwned, pd.coresOwned+proc.Tcore(msg.Amount), cores, CORE_IDLE)
	// Notify sleeping workers that they may be able to run procs now.
	go func() {
		for i := 0; i < msg.Amount; i++ {
			pd.stealChan <- true
		}
	}()
}

func (pd *Procd) removeCores(msg *resource.ResourceMsg) {
	cores := parseCoreInterval(msg)
	pd.adjustCoresOwned(pd.coresOwned, pd.coresOwned-proc.Tcore(msg.Amount), cores, CORE_BLOCKED)
}

func (pd *Procd) adjustCoresOwned(oldNCoresOwned, newNCoresOwned proc.Tcore, coresToMark []uint, newCoreStatus Tcorestatus) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.rebalanceProcs(oldNCoresOwned, newNCoresOwned, coresToMark, newCoreStatus)
	pd.sanityCheckCoreCountsL()
	pd.setCoreAffinityL()
}

// Rebalances procs across set of available cores. We allocate each proc a
// share of the owned cores proportional to their prior allocation, or the
// number of cores the proc requested, whichever is less. For simplicity, we
// currently move around all of the procs, even if they aren't having their
// cores revoked. In future, we should probably only move procs which
// absolutely need to release their cores.
func (pd *Procd) rebalanceProcs(oldNCoresOwned, newNCoresOwned proc.Tcore, coresToMark []uint, newCoreStatus Tcorestatus) {
	// Free all procs' cores.
	for _, p := range pd.runningProcs {
		pd.freeCoresL(p)
	}
	// After freeing old, used cores, mark cores according to their new status.
	pd.markCoresL(coresToMark, newCoreStatus)
	// Sanity check
	if pd.coresAvail != oldNCoresOwned {
		db.DFatalf("Mismatched num cores avail during rebalance: %v != %v", pd.coresAvail, oldNCoresOwned)
	}
	// Update the number of cores owned/available.
	pd.coresOwned = newNCoresOwned
	pd.coresAvail = newNCoresOwned
	toEvict := map[proc.Tpid]*LinuxProc{}
	// Calculate new core allocation for each proc, and allocate it cores. Some
	// of these procs may need to be evicted if there isn't enough space for
	// them.
	for pid, p := range pd.runningProcs {
		var newNCore proc.Tcore
		if p.attr.Ncore == 0 {
			// If this core didn't ask for dedicated cores, it can run on all cores.
			newNCore = newNCoresOwned
		} else {
			newNCore = proc.Tcore(len(p.cores)) * newNCoresOwned / oldNCoresOwned
			// Don't allocate more than the number of cores this proc initially asked
			// for.
			if newNCore > p.attr.Ncore {
				// XXX This seems to me like it could lead to some fishiness when
				// growing back after a shrink. One proc may not get all of its desired
				// cores back, while some of those cores may sit idle. It is simple,
				// though, so keep it for now.
				newNCore = p.attr.Ncore
			}
		}
		// If this proc would be allocated less than one core, slate it for
		// eviction, and don't alloc any cores.
		if newNCore < 1 {
			toEvict[pid] = p
		} else {
			// Resize the proc's core allocation.
			p.cores = make([]uint, newNCore)
			// Allocate cores to the proc.
			pd.allocCoresL(p)
			// Set the CPU affinity for this proc to match its new core allocation.
			p.setCpuAffinityL()
		}
	}
	// See if any of the procs to be evicted can still be squeezed in, in case
	// the "proportional allocation" strategy above left some cores unused.
	for pid, p := range toEvict {
		// If the proc fits...
		if p.attr.Ncore < pd.coresAvail {
			// Resize the proc.
			p.cores = make([]uint, p.attr.Ncore)
			// Allocate cores to the proc.
			pd.allocCoresL(p)
			// Set the CPU affinity for this proc to match its new corea llocation.
			p.setCpuAffinityL()
			delete(toEvict, pid)
		}
	}
	pd.evictProcsL(toEvict)
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
	pd.sanityCheckCoreCountsL()
}

// Set the status of a set of cores. Caller holds lock.
func (pd *Procd) markCoresL(cores []uint, status Tcorestatus) {
	for _, i := range cores {
		// If we are double-setting a core's status, it's probably a bug.
		if pd.coreBitmap[i] == status {
			debug.PrintStack()
			db.DFatalf("Error: Double-marked cores %v == %v", pd.coreBitmap[i], status)
		}
		pd.coreBitmap[i] = status
	}
}

func (pd *Procd) freeCores(p *LinuxProc) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.freeCoresL(p)
}

// Free a set of cores which was being used by a proc.
func (pd *Procd) freeCoresL(p *LinuxProc) {
	// If no cores were exclusively allocated to this proc, return immediately.
	if p.attr.Ncore == proc.C_DEF {
		return
	}

	pd.markCoresL(p.cores, CORE_IDLE)
	pd.coresAvail += proc.Tcore(len(p.cores))
	pd.sanityCheckCoreCountsL()
}

func parseCoreInterval(msg *resource.ResourceMsg) []uint {
	iv := np.MkInterval(0, 0)
	iv.Unmarshal(msg.Name)
	cores := make([]uint, msg.Amount)
	for i := uint(0); i < uint(msg.Amount); i++ {
		cores[i] = uint(iv.Start) + i
	}
	return cores
}

// Run a sanity check for our core resource accounting. Caller holds lock.
func (pd *Procd) sanityCheckCoreCountsL() {
	if pd.coresOwned > proc.Tcore(linuxsched.NCores) {
		db.DFatalf("Own more procd cores than there are cores on this machine: %v > %v", pd.coresOwned, linuxsched.NCores)
	}
	if pd.coresOwned <= 0 {
		db.DFatalf("Own too few cores: %v <= 0", pd.coresOwned)
	}
	if pd.coresAvail < 0 {
		db.DFatalf("Too few cores available: %v < 0", pd.coresAvail)
	}
	if pd.coresAvail > pd.coresOwned {
		db.DFatalf("More cores available than cores owned: %v > %v", pd.coresAvail, pd.coresOwned)
	}
}
