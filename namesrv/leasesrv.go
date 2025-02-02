package namesrv

import (
	"context"
	"time"

	"go.etcd.io/etcd/client/v3"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	leaseproto "sigmaos/ft/lease/proto"
	"sigmaos/namesrv/fsetcd"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
)

type LeaseSrv struct {
	lt *syncmap.SyncMap[sp.TclntId, clientv3.LeaseID]
	fs *fsetcd.FsEtcd
	lc clientv3.Lease
}

func newLeaseSrv(fs *fsetcd.FsEtcd) *LeaseSrv {
	return &LeaseSrv{
		lt: syncmap.NewSyncMap[sp.TclntId, clientv3.LeaseID](),
		fs: fs,
		lc: clientv3.NewLease(fs.Clnt()),
	}
}

func (ls *LeaseSrv) AskLease(ctx fs.CtxI, req leaseproto.AskReq, rep *leaseproto.AskRep) error {
	db.DPrintf(db.LEASESRV, "%v: AskLease %v", ctx.ClntId(), req.TTL)
	if lid, ok := ls.lt.Lookup(ctx.ClntId()); ok {
		rep.LeaseId = uint64(lid)
		return nil
	}
	start := time.Now()
	resp, err := ls.lc.Grant(context.TODO(), int64(req.TTL))
	if err != nil {
		return err
	}
	db.DPrintf(db.LEASESRV, "Grant lease lid %v lat %v", uint64(resp.ID), time.Since(start))
	rep.LeaseId = uint64(resp.ID)
	return nil
}

func (ls *LeaseSrv) Extend(ctx fs.CtxI, req leaseproto.ExtendReq, rep *leaseproto.ExtendRep) error {
	db.DPrintf(db.LEASESRV, "%v: Extend %v", ctx.ClntId(), sp.TleaseId(req.LeaseId))
	resp, err := ls.lc.KeepAliveOnce(context.TODO(), clientv3.LeaseID(req.LeaseId))
	if err != nil {
		return err
	}
	db.DPrintf(db.LEASESRV, "%v: Extend KeepAliveOnce [%v,%v]", ctx.ClntId(), resp.ID, resp.TTL)
	return nil
}

func (ls *LeaseSrv) End(ctx fs.CtxI, req leaseproto.ExtendReq, rep *leaseproto.ExtendRep) error {
	db.DPrintf(db.LEASESRV, "%v: End %v", ctx.ClntId(), sp.TleaseId(req.LeaseId))
	resp, err := ls.lc.Revoke(context.TODO(), clientv3.LeaseID(req.LeaseId))
	if err != nil {
		db.DPrintf(db.LEASESRV, "%v: End Revoke err: %v", ctx.ClntId(), err)
		return err
	}
	db.DPrintf(db.LEASESRV, "%v: End Revoke %v", ctx.ClntId(), resp)
	return nil
}
