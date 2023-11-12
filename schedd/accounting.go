package schedd

import (
	"sync/atomic"

	"sigmaos/proc"
	"sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
)

func (sd *Schedd) incRealmCnt(realm sp.Trealm) {
	if realm == "" {
		return
	}
	st := sd.getRealmStats(realm)
	atomic.AddInt64(&st.Running, 1)
	atomic.AddInt64(&st.TotalRan, 1)
}

func (sd *Schedd) decRealmCnt(p *proc.Proc) {
	// Don't count privileged procs
	if p.IsPrivileged() || p.GetRealm() == "" {
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
