package namesrv

import (
	"context"

	"go.etcd.io/etcd/client/v3"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/namesrv/fsetcd"
	leaseproto "sigmaos/lease/proto"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
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

func (ls *LeaseSrv) AskLease(ctx fs.CtxI, req leaseproto.AskRequest, rep *leaseproto.AskResult) error {
	db.DPrintf(db.LEASESRV, "%v: AskLease %v", ctx.ClntId(), req.TTL)
	if lid, ok := ls.lt.Lookup(ctx.ClntId()); ok {
		rep.LeaseId = uint64(lid)
		return nil
	}
	resp, err := ls.lc.Grant(context.TODO(), int64(req.TTL))
	if err != nil {
		return err
	}
	rep.LeaseId = uint64(resp.ID)
	return nil
}

func (ls *LeaseSrv) Extend(ctx fs.CtxI, req leaseproto.ExtendRequest, rep *leaseproto.ExtendResult) error {
	db.DPrintf(db.LEASESRV, "%v: Extend %v", ctx.ClntId(), sp.TleaseId(req.LeaseId))
	resp, err := ls.lc.KeepAliveOnce(context.TODO(), clientv3.LeaseID(req.LeaseId))
	if err != nil {
		return err
	}
	db.DPrintf(db.LEASESRV, "%v: Extend KeepAliveOnce [%v,%v]", ctx.ClntId(), resp.ID, resp.TTL)
	return nil
}

func (ls *LeaseSrv) End(ctx fs.CtxI, req leaseproto.ExtendRequest, rep *leaseproto.ExtendResult) error {
	db.DPrintf(db.LEASESRV, "%v: End %v", ctx.ClntId(), sp.TleaseId(req.LeaseId))
	resp, err := ls.lc.Revoke(context.TODO(), clientv3.LeaseID(req.LeaseId))
	if err != nil {
		db.DPrintf(db.LEASESRV, "%v: End Revoke err: %v", ctx.ClntId(), err)
		return err
	}
	db.DPrintf(db.LEASESRV, "%v: End Revoke %v", ctx.ClntId(), resp)
	return nil
}
