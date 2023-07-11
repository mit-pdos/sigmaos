package leasemgrsrv

import (
	leaseproto "sigmaos/lease/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type LeaseSrv struct {
	lt *syncmap.SyncMap[sp.TclntId, sp.TleaseId]
}

func NewLeaseSrv() *LeaseSrv {
	return &LeaseSrv{lt: syncmap.NewSyncMap[sp.TclntId, sp.TleaseId]()}
}

func (ls *LeaseSrv) AskLease(ctx fs.CtxI, req leaseproto.AskRequest, rep *leaseproto.AskResult) error {
	db.DPrintf(db.LEASESRV, "%v: AskLease %v req\n", ctx, req)
	return nil
}

func (ls *LeaseSrv) Extend(ctx fs.CtxI, req leaseproto.ExtendRequest, rep *leaseproto.ExtendResult) error {
	db.DPrintf(db.LEASESRV, "%v: Extend %v\n", ctx.ClntId(), req.LeaseId)
	return nil
}

func (ls *LeaseSrv) End(ctx fs.CtxI, req leaseproto.ExtendRequest, rep *leaseproto.ExtendResult) error {
	db.DPrintf(db.LEASESRV, "%v: End %v\n", ctx.ClntId(), req.LeaseId)
	return nil
}
