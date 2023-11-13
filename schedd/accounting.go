package schedd

import (
	"sync/atomic"

	"sigmaos/proc"
	"sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
)

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
	atomic.AddInt64(&st.Running, 1)
	atomic.AddInt64(&st.TotalRan, 1)
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
	atomic.AddInt64(&st.Running, -1)
}

func (sd *Schedd) getRealmStats(realm sp.Trealm) *proto.RealmStats {
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
			st = &proto.RealmStats{
				Running:  0,
				TotalRan: 0,
			}
			sd.scheddStats[realm] = st
		}
		// Demote to reader lock
		sd.realmMu.Unlock()
		sd.realmMu.RLock()
	}
	return st
}
