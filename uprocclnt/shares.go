package uprocclnt

import (
	"sigmaos/proc"
)

// Rebalance CPU shares when a proc runs.
func (updm *UprocdMgr) run(p *proc.Proc) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	// updm.allocUProc(uproc)
	// updm.RebalanceCPUShares()
}

// Rebalance CPU shares when a proc exits.
func (updm *UprocdMgr) exit(p *proc.Proc) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	// updm.allocUProc(uproc)
	// updm.RebalanceCPUShares()
}
