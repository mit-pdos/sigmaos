package schedd

import (
	"sync/atomic"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func (sd *Schedd) getPrefRealm() sp.Trealm {
	sd.realmMu.RLock()
	defer sd.realmMu.RUnlock()

	var minRealm sp.Trealm = sp.ROOTREALM
	var minCnt int64 = -1
	for r, c := range sd.realmCnts {
		// Ignore root realm.
		if r == sp.ROOTREALM {
			continue
		}
		cnt := atomic.LoadInt64(c)
		if cnt < minCnt || minCnt == -1 {
			minCnt = cnt
			minRealm = r
		}
	}
	return minRealm
}

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
