package clnt

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
func (pdm *ProcdMgr) startBalanceShares(p *proc.Proc) {
	pdm.mu.Lock()
	defer pdm.mu.Unlock()

	// Bail out early if dummy prog
	if p.GetProgram() == sp.DUMMY_PROG {
		return
	}

	switch p.GetType() {
	case proc.T_LC:
		rpcc := pdm.upcs[p.GetRealm()][p.GetType()]
		// Reset rpcc share amount to 0, since it was set to min before.
		if rpcc.share == MIN_SHARE {
			rpcc.share = 0
		}
		pdm.setShare(rpcc, rpcc.share+mcpuToShare(p.GetMcpu()))
	case proc.T_BE:
		pdm.balanceBEShares()
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}
}

// Rebalance CPU shares when a proc exits.
func (pdm *ProcdMgr) exitBalanceShares(p *proc.Proc) {
	pdm.mu.Lock()
	defer pdm.mu.Unlock()

	// Bail out early if dummy prog
	if p.GetProgram() == sp.DUMMY_PROG {
		return
	}

	switch p.GetType() {
	case proc.T_LC:
		rpcc := pdm.upcs[p.GetRealm()][p.GetType()]
		pdm.setShare(rpcc, rpcc.share-mcpuToShare(p.GetMcpu()))
	case proc.T_BE:
		// No need to readjust share.
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}
}

func (pdm *ProcdMgr) balanceBEShares() {
	// Equal share for each BE uprocd.
	cpuShare := BE_SHARES / Tshare(len(pdm.beProcds))
	for _, rpcc := range pdm.beProcds {
		// If the number of BE Procds has not changed, no rebalancing needs to
		// happen.
		if rpcc.share == cpuShare {
			continue
		}
		pdm.setShare(rpcc, cpuShare)
	}
	db.DPrintf(db.UPROCDMGR, "Rebalanced BE shares: %v", pdm.beProcds)
}

// Set a uprocd's CPU share, and RPC to the kernelsrv to adjust the shares.
func (pdm *ProcdMgr) setShare(rpcc *ProcClnt, share Tshare) {
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
	if err := pdm.kclnt.SetCPUShares(rpcc.pid, int64(share)); err != nil {
		db.DFatalf("Error SetCPUShares[%v] %v", rpcc.pid, err)
	}
	db.DPrintf(db.UPROCDMGR, "Set CPU share %v to %v", rpcc, share)
}

func mcpuToShare(mcpu proc.Tmcpu) Tshare {
	return (SHARE_PER_CORE * Tshare(mcpu)) / 1000
}
