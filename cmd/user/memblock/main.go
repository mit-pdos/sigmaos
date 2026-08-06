// memblock holds a node's scheduler memory budget until it is evicted, so that
// the node stops accepting procs.
//
// It deliberately allocates nothing. What keeps procs off a node is msched's
// memory accounting — the reservation its spawner set with Proc.SetMem, which
// msched deducts from the budget besched filters candidate nodes on — and that
// holds for as long as this proc is alive, whether or not it touches a byte.
// Actually consuming the memory would evict the page cache of whatever server the
// node was set aside to run, and risk the OOM killer taking it.
package main

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error newSigmaClnt: %v", err)
	}
	// Make the memblock dir.
	if err := sc.MkDir(sp.MEMBLOCK, 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		db.DFatalf("Unexpected mkdir err: %v", err)
	}
	// Register this memblocker, so that anyone can find out which nodes are
	// blocked. Tolerating "exists" because the marker names the kernel, not the
	// blocker: a caller needing more than one blocker to fill a node's budget
	// (util/memblock tops up when a proc is admitted between its query and its
	// blocker landing) would otherwise have every blocker after the first die here.
	if _, err := sc.Create(path.Join(sp.MEMBLOCK, sc.ProcEnv().KernelID), 0777, 0); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		db.DFatalf("Unexpected putfile err: %v", err)
	}
	db.DPrintf(db.ALWAYS, "Holding %v's memory budget until evicted", pe.GetKernelID())
	if err := sc.Started(); err != nil {
		db.DFatalf("Error started: %v", err)
	}
	if err := sc.WaitEvict(pe.GetPID()); err != nil {
		db.DFatalf("Err waitevict: %v", err)
	}
	sc.ClntExit(proc.NewStatus(proc.StatusEvicted))
}
