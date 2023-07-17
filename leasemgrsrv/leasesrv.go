package leasemgrsrv

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
	lt *syncmap.SyncMap[sp.TleaseId, *leaseInfo]
	et *ephemeralmap.EphemeralMap
	sync.Mutex
	nextLid sp.TleaseId
	ch      chan struct{}
}

type leaseInfo struct {
	sync.Mutex
	ttl  uint64
	time uint64
}

func NewLeaseSrv(et *ephemeralmap.EphemeralMap) *LeaseSrv {
	ls := &LeaseSrv{
		lt:      syncmap.NewSyncMap[sp.TleaseId, *leaseInfo](),
		et:      et,
		nextLid: 1,
		ch:      make(chan struct{}),
	}
	go ls.expirer()
	return ls
}

func (ls *LeaseSrv) AskLease(ctx fs.CtxI, req leaseproto.AskRequest, rep *leaseproto.AskResult) error {
	db.DPrintf(db.LEASESRV, "%v: AskLease req %v %v %v\n", ctx, req.ClntId, req.TTL)
	lid := ls.allocLid()
	ls.lt.Insert(lid, &leaseInfo{ttl: req.TTL, time: req.TTL})
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
	// XXX delete files that are associated with this lid
	return nil
}

func (ls *LeaseSrv) allocLid() sp.TleaseId {
	ls.Lock()
	defer ls.Unlock()

	lid := ls.nextLid
	ls.nextLid += 1
	return lid
}

func (ls *LeaseSrv) expirer() {
	select {
	case <-ls.ch:
		return
	case <-time.After(1 * time.Second):
		for _, li := range ls.lt.Values() {
			li.decTime()
		}
	}
}

func (li *leaseInfo) resetTTL() {
	li.Lock()
	defer li.Unlock()
	li.time = li.ttl
}

func (li *leaseInfo) decTime() {
	li.Lock()
	defer li.Unlock()

	li.ttl -= 1
	if li.ttl <= 0 {
		db.DPrintf(db.LEASESRV, "%v: Expire %v\n", li)
		// call memfs rm?
		// XXX delete files that are associated with this lid

		// ephemeral := ps.et.Values()
		// for _, po := range ephemeral {
		// 	db.DPrintf(db.ALWAYS, "Detach %v", po.Path())
		// 	// ps.removeObj(po.Ctx(), po.Obj(), po.Path())
		// }

	}
}
