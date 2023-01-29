package uprocclnt

import (
	db "sigmaos/debug"
	"sigmaos/proc"
)

type Tshare int64

// 1000 CPU shares should equal ~1 core.
//
// Resource distribution plan: * LC procs consume shares proportional to the
// number of cores they request.  * All BE procs, additively, consume shares
// proportional to 1 core. This means that if there are 2 realms, each running
// some BE procs, then each realm's BE procs will get .5 cores' worth of
// shares.
const (
	CPU_SHARES_PER_CORE Tshare = 1000
)

// Rebalance CPU shares when a proc runs.
func (updm *UprocdMgr) startBalanceShares(p *proc.Proc) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	switch p.GetType() {
	case proc.T_LC:
		// TODO
		db.DFatalf("Unimplemented")
	case proc.T_BE:
		updm.balanceBEShares()
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}

	// updm.allocUProc(uproc)
	// updm.RebalanceCPUShares()
}

// Rebalance CPU shares when a proc exits.
func (updm *UprocdMgr) exitBalanceShares(p *proc.Proc) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	// updm.allocUProc(uproc)
	// updm.RebalanceCPUShares()
}

func (updm *UprocdMgr) balanceBEShares() {
	// Equal shares for each BE uprocd.
	cpuShares := CPU_SHARES_PER_CORE / Tshare(len(updm.beUprocds))
	for _, pdc := range updm.beUprocds {
		// If the number of BE Uprocds has not changed, no rebalancing needs to
		// happen.
		if pdc.shares == cpuShares {
			continue
		}
		pdc.shares = cpuShares
		if err := updm.kclnt.SetCPUShares(pdc.pid, int64(cpuShares)); err != nil {
			db.DFatalf("Error SetCPUShares[%v] %v", pdc.pid, err)
		}
	}
}

//	// TODO: Set cpu shares differently for LC uprocds according to proc core requests.
//	var cpuShares int64
//	switch ptype {
//	case proc.T_LC:
//		cpuShares = CPU_SHARES_LC
//	case proc.T_BE:
//		cpuShares = CPU_SHARES_BE
//	default:
//		db.DFatalf("Unkown proc type: %v", ptype)
//	}
//	err = updm.kclnt.SetCPUShares(pid, cpuShares)
//	if err != nil {
//		return pid, err
//	}
//
