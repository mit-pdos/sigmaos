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
	SHARE_PER_CORE Tshare = 1000
)

// Rebalance CPU shares when a proc runs.
func (updm *UprocdMgr) startBalanceShares(p *proc.Proc) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	switch p.GetType() {
	case proc.T_LC:
		pdc := updm.pdcms[p.GetRealm()][p.GetType()]
		updm.setShare(pdc, pdc.share+coresToShare(p.GetNcore()))
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

	switch p.GetType() {
	case proc.T_LC:
		pdc := updm.pdcms[p.GetRealm()][p.GetType()]
		updm.setShare(pdc, pdc.share-coresToShare(p.GetNcore()))
	case proc.T_BE:
		// No need to readjust share.
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}
}

func (updm *UprocdMgr) balanceBEShares() {
	// Equal share for each BE uprocd.
	cpuShare := SHARE_PER_CORE / Tshare(len(updm.beUprocds))
	for _, pdc := range updm.beUprocds {
		// If the number of BE Uprocds has not changed, no rebalancing needs to
		// happen.
		if pdc.share == cpuShare {
			continue
		}
		updm.setShare(pdc, cpuShare)
	}
}

// Set a uprocd's CPU share, and RPC to the kernelsrv to adjust the shares.
func (updm *UprocdMgr) setShare(pdc *UprocdClnt, share Tshare) {
	pdc.share = share
	if err := updm.kclnt.SetCPUShares(pdc.pid, int64(share)); err != nil {
		db.DFatalf("Error SetCPUShares[%v] %v", pdc.pid, err)
	}
}

func coresToShare(cores proc.Tcore) Tshare {
	return Tshare(cores) * SHARE_PER_CORE
}
