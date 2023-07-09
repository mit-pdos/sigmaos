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
