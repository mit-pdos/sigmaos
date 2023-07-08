package leasemgrsrv

import (
	"sigmaos/ctx"
	"sigmaos/dir"
	"sigmaos/fs"
	leaseproto "sigmaos/lease/proto"
	"sigmaos/memfssrv"
	"sigmaos/protdevsrv"
	"sigmaos/sesssrv"
	// "sigmaos/serr"
	db "sigmaos/debug"
	"sigmaos/memfs"
	sp "sigmaos/sigmap"
)

type LeaseMgrSrv struct {
	pds *protdevsrv.ProtDevSrv
}

type LeaseSrv struct {
}

// goal: add rpc service to existing memfs
// need a memfs because we need the path lock table
// so we need to pass in the pathname for where the rpc service will be
// two memfssrv: the sessrv one and the one for protocol server, which
// we want to mount into sesssrv
func NewLeaseMgrSrv(uname sp.Tuname, srv *sesssrv.SessSrv) (*LeaseMgrSrv, error) {
	d := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	srv.Mount(sp.LEASESRV, d.(*dir.DirImpl))
	mfs := memfssrv.MakeMemFsSrv(uname, srv)
	lsrv := &LeaseSrv{}
	pds, err := protdevsrv.MakeProtDevSrvMemFs(mfs, sp.LEASESRV, lsrv)
	if err != nil {
		return nil, err
	}
	return &LeaseMgrSrv{pds: pds}, nil
}

func (ls *LeaseSrv) AskLease(ctx fs.CtxI, req leaseproto.AskRequest, rep *leaseproto.AskResult) error {
	db.DPrintf(db.LEASESRV, "%v: AskLease %v req\n", ctx, req)
	return nil
}
