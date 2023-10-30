package schedd

import (
	"sync/atomic"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func (sd *Schedd) incRealmCnt(realm sp.Trealm) {
	if realm == "" {
		return
	}
	cnt := sd.getRealmCnt(realm)
	atomic.AddInt64(cnt, 1)
}

func (sd *Schedd) decRealmCnt(p *proc.Proc) {
	// Don't count privileged procs
	if p.IsPrivileged() || p.GetRealm() == "" {
		return
	}
	cnt := sd.getRealmCnt(p.GetRealm())
	atomic.AddInt64(cnt, -1)
}

func (sd *Schedd) getRealmCnt(realm sp.Trealm) *int64 {
	sd.realmMu.RLock()
	defer sd.realmMu.RUnlock()

	cnt, ok := sd.realmCnts[realm]
	if !ok {
		// Promote to writer lock.
		sd.realmMu.RUnlock()
		sd.realmMu.Lock()
		// Check if the count was created during lock promotion.
		cnt, ok = sd.realmCnts[realm]
		if !ok {
			var cnt2 int64 = 0
			cnt = &cnt2
			sd.realmCnts[realm] = cnt
		}
		// Demote to reader lock
		sd.realmMu.Unlock()
		sd.realmMu.RLock()
	}
	return cnt
}
