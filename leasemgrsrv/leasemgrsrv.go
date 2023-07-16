package leasemgrsrv

import (
	"sigmaos/ctx"
	"sigmaos/dir"
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

func NewLeaseMgrSrv(uname sp.Tuname, srv *sesssrv.SessSrv, svc any) (*LeaseMgrSrv, error) {
	db.DPrintf(db.LEASESRV, "NewLeaseMgrSrv: %v\n", svc)
	d := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	srv.Mount(sp.LEASESRV, d.(*dir.DirImpl))
	mfs := memfssrv.MakeMemFsSrv(uname, "", srv)
	pds, err := protdevsrv.MakeProtDevSrvMemFs(mfs, sp.LEASESRV, svc)
	if err != nil {
		return nil, err
	}
	return &LeaseMgrSrv{pds: pds}, nil
}