package leasemgrsrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/memfs"
	"sigmaos/memfssrv"
	"sigmaos/protdevsrv"
	"sigmaos/sesssrv"
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
