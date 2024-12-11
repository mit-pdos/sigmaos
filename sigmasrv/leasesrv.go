package sigmasrv

import (
	"sync"
	"time"

	leaseproto "sigmaos/ft/lease/proto"

	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/sigmasrv/memfssrv"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/spproto/srv/leasedmap"
	"sigmaos/util/syncmap"
)

type LeaseSrv struct {
	mfs     *memfssrv.MemFs
	lt      *syncmap.SyncMap[sp.TleaseId, *lease]
	lm      *leasedmap.LeasedMap
	mu      sync.Mutex
	nextLid sp.TleaseId
	ch      chan struct{}
}

type lease struct {
	sync.Mutex
	ttl  uint64
	time uint64
	lid  sp.TleaseId
}

func newLeaseSrv(mfs *memfssrv.MemFs) *LeaseSrv {
	ls := &LeaseSrv{
		mfs:     mfs,
		lt:      syncmap.NewSyncMap[sp.TleaseId, *lease](),
		lm:      mfs.Leasedmap(),
		nextLid: 1,
		ch:      make(chan struct{}),
	}
	go ls.expirer()
	return ls
}

func (ls *LeaseSrv) AskLease(ctx fs.CtxI, req leaseproto.AskRequest, rep *leaseproto.AskResult) error {
	lid := ls.allocLid()
	ls.lt.Insert(lid, &lease{ttl: req.TTL, time: req.TTL, lid: lid})
	rep.LeaseId = uint64(lid)
	db.DPrintf(db.LEASESRV, "%v: AskLease req %v %v: lid %v\n", ctx, req.ClntId, req.TTL, lid)
	return nil
}

func (ls *LeaseSrv) Extend(ctx fs.CtxI, req leaseproto.ExtendRequest, rep *leaseproto.ExtendResult) error {
	db.DPrintf(db.LEASESRV, "%v: Extend %v\n", ctx.ClntId(), sp.TleaseId(req.LeaseId))
	li, ok := ls.lt.Lookup(sp.TleaseId(req.LeaseId))
	if !ok {
		return serr.NewErr(serr.TErrNotfound, req.LeaseId)
	}
	li.resetTTL()
	return nil
}

func (ls *LeaseSrv) End(ctx fs.CtxI, req leaseproto.ExtendRequest, rep *leaseproto.ExtendResult) error {
	lid := sp.TleaseId(req.LeaseId)
	db.DPrintf(db.LEASESRV, "%v: End %v\n", ctx.ClntId(), lid)
	_, ok := ls.lt.Lookup(lid)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, req.LeaseId)
	}
	ls.lt.Delete(lid)
	ls.expire(lid)
	return nil
}

func (ls *LeaseSrv) stop() {
	ls.ch <- struct{}{}
}

func (ls *LeaseSrv) allocLid() sp.TleaseId {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	lid := ls.nextLid
	ls.nextLid += 1
	return lid
}

// Delete files that are associated with lid
func (ls *LeaseSrv) expire(lid sp.TleaseId) {
	pns := ls.lm.Expired(lid)
	db.DPrintf(db.ALWAYS, "expire %v %v\n", lid, pns)
	for _, pn := range pns {
		ls.mfs.Remove(pn)
	}
}

func (ls *LeaseSrv) expirer() {
	db.DPrintf(db.LEASESRV, "expirer running")
	for {
		select {
		case <-ls.ch:
			return
		case <-time.After(1 * time.Second):
			for _, li := range ls.lt.Values() {
				db.DPrintf(db.LEASESRV, "expirer dec %v", li)
				if li.decTime() {
					ls.expire(li.lid)
				}
			}
		}
	}
	db.DPrintf(db.LEASESRV, "expirer done")
}

func (l *lease) resetTTL() {
	l.Lock()
	defer l.Unlock()
	l.time = l.ttl
}

func (l *lease) decTime() bool {
	l.Lock()
	defer l.Unlock()

	l.time -= 1
	if l.time <= 0 {
		return true
	}
	return false
}
