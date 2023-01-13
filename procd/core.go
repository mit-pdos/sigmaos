package procd

import (
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/proc"
)

// Allocate cores to a proc. Caller holds lock.
func (pd *Procd) allocCoresL(n proc.Tcore) {
	if n > pd.coresAvail {
		debug.PrintStack()
		pd.perf.Done()
		db.DFatalf("Alloc too many cores %v > %v", n, pd.coresAvail)
	}
	pd.coresAvail -= n
	pd.sanityCheckCoreCountsL()
}

func (pd *Procd) freeCores(p *LinuxProc) {
	pd.Lock()
	defer pd.Unlock()

	pd.freeCoresL(p)
}

// Free a set of cores which was being used by a proc.
func (pd *Procd) freeCoresL(p *LinuxProc) {
	// If no cores were exclusively allocated to this proc, return immediately.
	if p.attr.GetNcore() == proc.C_DEF {
		return
	}

	pd.sanityCheckCoreCountsL()
}

// Run a sanity check for our core resource accounting. Caller holds lock.
func (pd *Procd) sanityCheckCoreCountsL() {

	if pd.coresAvail < 0 {
		pd.perf.Done()
		db.DFatalf("Too few cores available: %v < 0", pd.coresAvail)
	}
	if pd.coresAvail > proc.Tcore(linuxsched.NCores) {
		debug.PrintStack()
		pd.perf.Done()
		db.DFatalf("More cores available than cores on node: %v > %v", pd.coresAvail, linuxsched.NCores)
	}
}
