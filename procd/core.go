package procd

import (
	"math"
	"runtime/debug"
	"time"

	db "ulambda/debug"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/resource"
	"ulambda/stats"
)

type Tcorestatus uint8

const (
	CORE_AVAILABLE Tcorestatus = iota
	CORE_BLOCKED               // Not for use by this procd's procs.
)

func (st Tcorestatus) String() string {
	switch st {
	case CORE_AVAILABLE:
		return "CORE_AVAILABLE"
	case CORE_BLOCKED:
		return "CORE_BLOCKED"
	default:
		db.DFatalf("Unrecognized core status")
	}
	return ""
}

func (pd *Procd) initCores(grantedCoresIv string) {
	grantedCores := np.MkInterval(0, 0)
	grantedCores.Unmarshal(grantedCoresIv)
	// First, revoke access to all cores.
	allCoresIv := np.MkInterval(0, np.Toffset(linuxsched.NCores))
	revokeMsg := resource.MakeResourceMsg(resource.Trequest, resource.Tcore, allCoresIv.String(), int(linuxsched.NCores))
	pd.removeCores(revokeMsg)

	// Then, enable access to the granted core interval.
	grantMsg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, grantedCores.String(), int(grantedCores.Size()))
	pd.addCores(grantMsg)
}

func (pd *Procd) addCores(msg *resource.ResourceMsg) {
	cores := parseCoreInterval(msg.Name)
	pd.adjustCoresOwned(pd.coresOwned, pd.coresOwned+proc.Tcore(msg.Amount), cores, CORE_AVAILABLE)
	db.DPrintf(db.ALWAYS, "Added cores to procd")
	// Notify sleeping workers that they may be able to run procs now.
	go func() {
		for i := 0; i < msg.Amount; i++ {
			pd.stealChan <- true
		}
	}()
}

func (pd *Procd) removeCores(msg *resource.ResourceMsg) {
	cores := parseCoreInterval(msg.Name)
	pd.adjustCoresOwned(pd.coresOwned, pd.coresOwned-proc.Tcore(msg.Amount), cores, CORE_BLOCKED)
}

func (pd *Procd) adjustCoresOwned(oldNCoresOwned, newNCoresOwned proc.Tcore, coresToMark []uint, newCoreStatus Tcorestatus) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Mark cores according to their new status.
	pd.markCoresL(coresToMark, newCoreStatus)
	// Set the new procd core affinity.
	pd.setCoreAffinityL()
	// Rebalance procs given new cores.
	pd.rebalanceProcs(oldNCoresOwned, newNCoresOwned, coresToMark, newCoreStatus)
	pd.sanityCheckCoreCountsL()
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
		newNCore := p.attr.Ncore * newNCoresOwned / oldNCoresOwned
		// Don't allocate more than the number of cores this proc initially asked
		// for.
		if newNCore > p.attr.Ncore {
			// XXX This seems to me like it could lead to some fishiness when
			// growing back after a shrink. One proc may not get all of its desired
			// cores back, while some of those cores may sit idle. It is simple,
			// though, so keep it for now.
			newNCore = p.attr.Ncore
		}
		// If this proc would be allocated less than one core, slate it for
		// eviction, and don't alloc any cores.
		if newNCore < 1 {
			toEvict[pid] = p
		} else {
			// Resize the proc's core allocation.
			// Allocate cores to the proc.
			pd.allocCoresL(p, newNCore)
			// Set the CPU affinity for this proc to match procd.
			p.setCpuAffinityL()
		}
	}
	// See if any of the procs to be evicted can still be squeezed in, in case
	// the "proportional allocation" strategy above left some cores unused.
	for pid, p := range toEvict {
		// If the proc fits...
		if p.attr.Ncore < pd.coresAvail {
			// Allocate cores to the proc.
			pd.allocCoresL(p, p.attr.Ncore)
			// Set the CPU affinity for this proc to match procd.
			p.setCpuAffinityL()
			delete(toEvict, pid)
		}
	}
	pd.evictProcsL(toEvict)
}

// Rate-limit how quickly we claim BE procs, since utilization statistics will
// take a while to update while claimed procs start. Return true if check
// passes and proc can be claimed.
//
// We claim a maximum of BE_PROC_OVERSUBSCRIPTION_RATE
// procs per underutilized core core per claim interval, where a claim interval
// is the length of ten CPU util samples.
func (pd *Procd) procClaimRateLimitCheck(util float64, p *proc.Proc) bool {
	timeBetweenUtilSamples := time.Duration(1000/np.Conf.Perf.CPU_UTIL_SAMPLE_HZ) * time.Millisecond
	// Check if we have moved onto the next interval (interval is currently 10 *
	// utilization sample rate).
	if time.Since(pd.procClaimTime) > 10*timeBetweenUtilSamples {
		pd.procClaimTime = time.Now()
		// We try to estimate the amount of "room" available for claiming new procs.
		pd.netProcsClaimed = proc.Tcore(math.Round(float64(pd.coresOwned) * util / 100.0))
		// If a proc is downloading, it's utilization won't have been measured yet.
		// Adding this to the number of procs claimed is perhaps a little too
		// conservative (we may double-count if the proc which is downloading was
		// also claimed in this epoch), but this should only happen the first time
		// a proc is downloaded, which should not be often.
		pd.netProcsClaimed += pd.procsDownloading
	}
	// If we have claimed < BE_PROC_OVERSUBSCRIPTION_RATE
	// procs per core during the last claim interval, the rate limit check
	// passes.
	maxOversub := proc.Tcore(np.Conf.Procd.BE_PROC_OVERSUBSCRIPTION_RATE * float64(pd.coresOwned))
	if pd.netProcsClaimed < maxOversub {
		return true
	}
	db.DPrintf("PROCD", "Failed proc claim rate limit check: %v > %v for proc %v", pd.netProcsClaimed, maxOversub, p)
	return false
}

func (pd *Procd) overloaded(util float64, cload stats.Tload) bool {
	// If utilization is growing very quickly, we may be overloaded.
	if cload[0]-cload[1] >= 10.0 && cload[1]-cload[2] >= 10.0 {
		return true
	}
	t := np.Conf.Procd.BE_PROC_CLAIM_CPU_THRESHOLD
	// If we have a history of high utilization...
	if cload[0] >= t && cload[1] >= t {
		return true
	}
	// If there is a sudden drop in CPU utilization...
	if cload[0]-util > 20.0 {
		return true
	}
	return false
	/*  && !(cload[0] >= 95.0 && cload[1] >= 95.0 && cload[2] >= 95.0) && !(util-cload[0] >= 20.0) && */
}

// Check if this procd has enough cores to run proc p. Caller holds lock.
func (pd *Procd) hasEnoughCores(p *proc.Proc) bool {
	// If this is an LC proc, check that we have enough cores.
	if p.Type == proc.T_LC {
		// If we have enough cores to run this job...
		if pd.coresAvail >= p.Ncore {
			return true
		}
		db.DPrintf("PROCD", "Don't have enough LC cores (%v) for %v", pd.coresAvail, p)
	} else {
		// Otherwise, determine whether or not we can run the proc based on
		// utilization. If utilization is below a certain threshold, take the proc.
		util := pd.GetStats().GetUtil()
		load := pd.GetStats().GetLoad()
		cload := pd.GetStats().GetCustomLoad()
		rlc := pd.procClaimRateLimitCheck(util, p)
		if util < np.Conf.Procd.BE_PROC_CLAIM_CPU_THRESHOLD && !pd.overloaded(util, cload) && rlc {
			progs := make([]string, 0, len(pd.runningProcs))
			for _, p := range pd.runningProcs {
				progs = append(progs, p.attr.Program)
			}
			db.DPrintf(db.ALWAYS, "Claimed BE proc: util %v Linux load %v Custom load %v rate-limit check %v proc %v, running %v", util, load, cload, rlc, p.Program, progs)
			return true
		}
		db.DPrintf("PROCD", "Couldn't claim BE proc: util %v rate-limit check %v proc %v", util, rlc, p)
	}
	return false
}

// Allocate cores to a proc. Caller holds lock.
func (pd *Procd) allocCoresL(p *LinuxProc, n proc.Tcore) {
	p.coresAlloced = n
	pd.coresAvail -= n
	pd.netProcsClaimed++
	pd.sanityCheckCoreCountsL()
}

// Set the status of a set of cores. Caller holds lock.
func (pd *Procd) markCoresL(cores []uint, status Tcorestatus) {
	for _, i := range cores {
		// If we are double-setting a core's status, it's probably a bug.
		if pd.coreBitmap[i] == status {
			debug.PrintStack()
			db.DFatalf("Error (noded:%v): Double-marked cores %v == %v", proc.GetNodedId(), pd.coreBitmap[i], status)
		}
		pd.coreBitmap[i] = status
	}
}

func (pd *Procd) freeCores(p *LinuxProc) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.freeCoresL(p)
	if p.attr.Type != proc.T_LC {
		if pd.netProcsClaimed > 0 {
			pd.netProcsClaimed--
		}
	}
}

// Free a set of cores which was being used by a proc.
func (pd *Procd) freeCoresL(p *LinuxProc) {
	// If no cores were exclusively allocated to this proc, return immediately.
	if p.attr.Ncore == proc.C_DEF {
		return
	}

	pd.coresAvail += p.coresAlloced
	p.coresAlloced = 0
	pd.sanityCheckCoreCountsL()
}

func parseCoreInterval(ivStr string) []uint {
	iv := np.MkInterval(0, 0)
	iv.Unmarshal(ivStr)
	cores := make([]uint, iv.Size())
	for i := uint(0); i < uint(iv.Size()); i++ {
		cores[i] = uint(iv.Start) + i
	}
	return cores
}

// Run a sanity check for our core resource accounting. Caller holds lock.
func (pd *Procd) sanityCheckCoreCountsL() {
	if pd.coresOwned > proc.Tcore(linuxsched.NCores) {
		db.DFatalf("Own more procd cores than there are cores on this machine: %v > %v", pd.coresOwned, linuxsched.NCores)
	}
	if pd.coresOwned < 0 {
		db.DFatalf("Own too few cores: %v <= 0", pd.coresOwned)
	}
	if pd.coresAvail < 0 {
		db.DFatalf("Too few cores available: %v < 0", pd.coresAvail)
	}
	if pd.coresAvail > pd.coresOwned {
		debug.PrintStack()
		db.DFatalf("More cores available than cores owned: %v > %v", pd.coresAvail, pd.coresOwned)
	}
}
