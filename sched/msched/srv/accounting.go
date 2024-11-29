package srv

import (
	"sync/atomic"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type realmStats struct {
	running  atomic.Int64
	totalRan atomic.Int64
}

func (msched *MSched) incRealmStats(p *proc.Proc) {
	// Don't count named or other privileged procs.
	if p.IsPrivileged() || p.GetRealm() == "" || p.GetProgram() == "named" {
		return
	}
	// For now, ignore MR coord
	if p.GetProgram() == "mr-coord" {
		return
	}
	st := msched.getRealmStats(p.GetRealm())
	st.running.Add(1)
	st.totalRan.Add(1)
}

func (msched *MSched) decRealmStats(p *proc.Proc) {
	// Don't count privileged procs
	if p.IsPrivileged() || p.GetRealm() == "" {
		return
	}
	// For now, ignore MR coord
	if p.GetProgram() == "mr-coord" {
		return
	}
	st := msched.getRealmStats(p.GetRealm())
	st.running.Add(-1)
}

func (msched *MSched) getRealmStats(realm sp.Trealm) *realmStats {
	msched.realmMu.RLock()
	defer msched.realmMu.RUnlock()

	st, ok := msched.scheddStats[realm]
	if !ok {
		// Promote to writer lock.
		msched.realmMu.RUnlock()
		msched.realmMu.Lock()
		// Check if the count was created during lock promotion.
		st, ok = msched.scheddStats[realm]
		if !ok {
			st = &realmStats{}
			msched.scheddStats[realm] = st
		}
		// Demote to reader lock
		msched.realmMu.Unlock()
		msched.realmMu.RLock()
	}
	return st
}
