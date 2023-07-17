package memfssrv

import (
	"sync"
	"time"

	leaseproto "sigmaos/lease/proto"

	db "sigmaos/debug"
	"sigmaos/ephemeralmap"
	"sigmaos/fs"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type LeaseSrv struct {
	mfs     *MemFs
	lt      *syncmap.SyncMap[sp.TleaseId, *leaseInfo]
	et      *ephemeralmap.EphemeralMap
	mu      sync.Mutex
	nextLid sp.TleaseId
	ch      chan struct{}
}

type leaseInfo struct {
	sync.Mutex
	ttl  uint64
	time uint64
	lid  sp.TleaseId
}

func NewLeaseSrv(mfs *MemFs) *LeaseSrv {
	ls := &LeaseSrv{
		mfs:     mfs,
		lt:      syncmap.NewSyncMap[sp.TleaseId, *leaseInfo](),
		et:      mfs.GetEphemeralMap(),
		nextLid: 1,
		ch:      make(chan struct{}),
	}
	go ls.expirer()
	return ls
}

func (ls *LeaseSrv) AskLease(ctx fs.CtxI, req leaseproto.AskRequest, rep *leaseproto.AskResult) error {
	db.DPrintf(db.LEASESRV, "%v: AskLease req %v %v\n", ctx, req.ClntId, req.TTL)
	lid := ls.allocLid()
	ls.lt.Insert(lid, &leaseInfo{ttl: req.TTL, time: req.TTL, lid: lid})
	rep.LeaseId = uint64(lid)
	return nil
}

func (ls *LeaseSrv) Extend(ctx fs.CtxI, req leaseproto.ExtendRequest, rep *leaseproto.ExtendResult) error {
	db.DPrintf(db.LEASESRV, "%v: Extend %v\n", ctx.ClntId(), req.LeaseId)
	li, ok := ls.lt.Lookup(sp.TleaseId(req.LeaseId))
	if !ok {
		return serr.MkErr(serr.TErrNotfound, req.LeaseId)
	}
	li.resetTTL()
	return nil
}

func (ls *LeaseSrv) End(ctx fs.CtxI, req leaseproto.ExtendRequest, rep *leaseproto.ExtendResult) error {
	db.DPrintf(db.LEASESRV, "%v: End %v\n", ctx.ClntId(), req.LeaseId)
	lid := sp.TleaseId(req.LeaseId)
	_, ok := ls.lt.Lookup(lid)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, req.LeaseId)
	}
	ls.lt.Delete(lid)
	ls.expire(lid)
	return nil
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
	pns := ls.et.Expire(lid)
	db.DPrintf(db.LEASESRV, "%v: expire %v %v\n", lid, pns)
	for _, pn := range pns {
		ls.mfs.Remove(pn)
	}
}

func (ls *LeaseSrv) expirer() {
	select {
	case <-ls.ch:
		return
	case <-time.After(1 * time.Second):
		for _, li := range ls.lt.Values() {
			if li.decTime() {
				ls.expire(li.lid)
			}
		}
	}
}

func (li *leaseInfo) resetTTL() {
	li.Lock()
	defer li.Unlock()
	li.time = li.ttl
}

func (li *leaseInfo) decTime() bool {
	li.Lock()
	defer li.Unlock()

	li.ttl -= 1
	if li.ttl <= 0 {
		return true
	}
	return false
}
