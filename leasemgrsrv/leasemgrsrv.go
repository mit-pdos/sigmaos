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

func NewLeaseSrvSvc(uname sp.Tuname, srv *sesssrv.SessSrv, svc any) (*protdevsrv.ProtDevSrv, error) {
	db.DPrintf(db.LEASESRV, "NewLeaseMgrSrv: %v\n", svc)
	d := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	srv.Mount(sp.LEASESRV, d.(*dir.DirImpl))
	mfs := memfssrv.MakeMemFsSrv(uname, "", srv)
	pds, err := protdevsrv.MakeProtDevSrvMemFs(mfs, sp.LEASESRV, svc)
	if err != nil {
		return nil, err
	}
	return pds, nil
}

func NewLeaseMgrSrv1(uname sp.Tuname, srv *sesssrv.SessSrv, svc any) (*protdevsrv.ProtDevSrv, error) {
	db.DPrintf(db.LEASESRV, "NewLeaseMgrSrv: %v\n", svc)
	d := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	srv.Mount(sp.LEASESRV, d.(*dir.DirImpl))
	mfs := memfssrv.MakeMemFsSrv(uname, "", srv)
	pds, err := protdevsrv.MakeProtDevSrvMemFs(mfs, sp.LEASESRV, svc)
	if err != nil {
		return nil, err
	}
	return pds, nil
}

func NewLeaseSrv(mfs *memfssrv.MemFs) error {
	db.DPrintf(db.LEASESRV, "NewLeaseSrv\n")
	lsrv := memfssrv.NewLeaseSrv(mfs)
	if _, err := mfs.Create(sp.LEASESRV, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	_, err := protdevsrv.MakeProtDevSrvMemFs(mfs, sp.LEASESRV, lsrv)
	if err != nil {
		return err
	}
	return nil
}
