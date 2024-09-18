package uprocclnt

import (
	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type Tshare int64

// 1000 CPU shares should equal ~1 core.
//
// Resource distribution plan:
// * LC procs consume shares proportional to the number of cores they request.
// * All BE procs, additively, consume shares proportional to 1 core. This
// means that if there are 2 realms, each running some BE procs, then each
// realm's BE procs will get .5 cores' worth of shares.

const (
	SHARE_PER_CORE Tshare = 1000
	BE_SHARES             = 50 // shares split by BE procs.
	MIN_SHARE             = 5
)

// Rebalance CPU shares when a proc runs.
func (updm *UprocdMgr) startBalanceShares(p *proc.Proc) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	// Bail out early if dummy prog
	if p.GetProgram() == sp.DUMMY_PROG {
		return
	}

	switch p.GetType() {
	case proc.T_LC:
		rpcc := updm.upcs[p.GetRealm()][p.GetType()]
		// Reset rpcc share amount to 0, since it was set to min before.
		if rpcc.share == MIN_SHARE {
			rpcc.share = 0
		}
		updm.setShare(rpcc, rpcc.share+mcpuToShare(p.GetMcpu()))
	case proc.T_BE:
		updm.balanceBEShares()
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}
}

// Rebalance CPU shares when a proc exits.
func (updm *UprocdMgr) exitBalanceShares(p *proc.Proc) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	// Bail out early if dummy prog
	if p.GetProgram() == sp.DUMMY_PROG {
		return
	}

	switch p.GetType() {
	case proc.T_LC:
		rpcc := updm.upcs[p.GetRealm()][p.GetType()]
		updm.setShare(rpcc, rpcc.share-mcpuToShare(p.GetMcpu()))
	case proc.T_BE:
		// No need to readjust share.
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}
}

func (updm *UprocdMgr) balanceBEShares() {
	// Equal share for each BE uprocd.
	cpuShare := BE_SHARES / Tshare(len(updm.beUprocds))
	for _, rpcc := range updm.beUprocds {
		// If the number of BE Uprocds has not changed, no rebalancing needs to
		// happen.
		if rpcc.share == cpuShare {
			continue
		}
		updm.setShare(rpcc, cpuShare)
	}
	db.DPrintf(db.UPROCDMGR, "Rebalanced BE shares: %v", updm.beUprocds)
}

// Set a uprocd's CPU share, and RPC to the kernelsrv to adjust the shares.
func (updm *UprocdMgr) setShare(rpcc *UprocdClnt, share Tshare) {
	if share < MIN_SHARE {
		// BE realms should not get <.1 cores.
		if rpcc.ptype == proc.T_BE {
			db.DFatalf("Assign %v share to BE uprocd", share)
		}
		// If the uprocd is an LC uprocd, and it isn't running and procs which
		// request cores, then set its share to .1 core.
		share = MIN_SHARE
	}
	// If the share isn't changing, return.
	if rpcc.share == share {
		db.DPrintf(db.UPROCDMGR, "Skip setting CPU share for %v: no change", rpcc, share)
		return
	}
	rpcc.share = share
	if rpcc.share > 10000 {
		db.DFatalf("Share outside of cgroupsv2 range [1,10000]: %v\n%v", rpcc.share, string(debug.Stack()))
	}
	if err := updm.kclnt.SetCPUShares(rpcc.pid, int64(share)); err != nil {
		db.DFatalf("Error SetCPUShares[%v] %v", rpcc.pid, err)
	}
	db.DPrintf(db.UPROCDMGR, "Set CPU share %v to %v", rpcc, share)
}

func mcpuToShare(mcpu proc.Tmcpu) Tshare {
	return (SHARE_PER_CORE * Tshare(mcpu)) / 1000
}
