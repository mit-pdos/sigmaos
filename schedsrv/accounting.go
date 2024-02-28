package schedsrv

import (
	"sync/atomic"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type realmStats struct {
	running  atomic.Int64
	totalRan atomic.Int64
}

func (sd *Schedd) incRealmStats(p *proc.Proc) {
	// Don't count named or other privileged procs.
	if p.IsPrivileged() || p.GetRealm() == "" || p.GetProgram() == "named" {
		return
	}
	// For now, ignore MR coord
	if p.GetProgram() == "mr-coord" {
		return
	}
	st := sd.getRealmStats(p.GetRealm())
	st.running.Add(1)
	st.totalRan.Add(1)
}

func (sd *Schedd) decRealmStats(p *proc.Proc) {
	// Don't count privileged procs
	if p.IsPrivileged() || p.GetRealm() == "" {
		return
	}
	// For now, ignore MR coord
	if p.GetProgram() == "mr-coord" {
		return
	}
	st := sd.getRealmStats(p.GetRealm())
	st.running.Add(-1)
}

func (sd *Schedd) getRealmStats(realm sp.Trealm) *realmStats {
	sd.realmMu.RLock()
	defer sd.realmMu.RUnlock()

	st, ok := sd.scheddStats[realm]
	if !ok {
		// Promote to writer lock.
		sd.realmMu.RUnlock()
		sd.realmMu.Lock()
		// Check if the count was created during lock promotion.
		st, ok = sd.scheddStats[realm]
		if !ok {
			st = &realmStats{}
			sd.scheddStats[realm] = st
		}
		// Demote to reader lock
		sd.realmMu.Unlock()
		sd.realmMu.RLock()
	}
	return st
}
