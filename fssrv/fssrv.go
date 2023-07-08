package fssrv

import (
	//db "sigmaos/debug"
	//"sigmaos/fs"
	//"sigmaos/fsetcd"
	//"sigmaos/fslibsrv"
	"sigmaos/leasemgrsrv"
	//"sigmaos/memfssrv"
	//"sigmaos/repl"
	"sigmaos/sesssrv"
	//"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type FsSrv struct {
	lms *leasemgrsrv.LeaseMgrSrv
}

func NewFsSrv(uname sp.Tuname, srv *sesssrv.SessSrv) (*FsSrv, error) {
	lms, err := leasemgrsrv.NewLeaseMgrSrv(uname, srv)
	if err != nil {
		return nil, err
	}
	return &FsSrv{lms: lms}, nil
}

// func (fssrv *FsSrv) MakeMemFs(pn string, name sp.Tuname) (*memfssrv.MemFs, error) {
// 	return fssrv.MakeMemFsPort(pn, ":0", uname)
// }

// // Make an MemFs for a specific port and advertise it at pn
// func (fssrv *FsSrv) MakeMemFsPort(pn, port string, uname sp.Tuname) (*memfssrv.MemFs, error) {
// 	sc, err := sigmaclnt.MkSigmaClnt(uname)
// 	if err != nil {
// 		return nil, err
// 	}
// 	db.DPrintf(db.PORT, "MakeMemFsPort %v %v\n", pn, port)
// 	fs, err := MakeMemFsPortClnt(pn, port, sc)
// 	return fs, err
// }

// // Make an MemFs for a specific port and client, and advertise it at
// // pn
// func (fssrv *FsSrv) MakeMemFsPortClnt(pn, port string, sc *sigmaclnt.SigmaClnt) (*memfssrv.MemFs,, error) {
// 	root := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
// 	srv, err := fssrv.MakeReplServerFsl(root, pn, port, sc)
// 	if err != nil {
// 		return nil, err
// 	}
// 	mfs := fslibsrv.MakeMemFsSrv(sp.Tuname(pn), srv)
// 	return mfs, nil
// }

// func (fssrv *FsSrv) MakeReplServerFsl(root fs.Dir, addr string, path string, sc *sigmaclnt.SigmaClnt, config repl.Config) (*sesssrv.SessSrv, error) {
// 	sesssrv, err := fslibsrv.MakeReplServerFsl(root, addr, path, sc, config)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(path) > 0 {
// 		mnt := sp.MkMountServer(sesssrv.MyAddr())
// 		db.DPrintf(db.BOOT, "Advertise %s at %v\n", path, mnt)
// 		lid, err := sc.LeaseMgrClnt.AskLease(path, fsetcd.LeaseTTL)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if err := sc.MkMountSymlink(path, mnt, lid); err != nil {
// 			return nil, err
// 		}
// 	}
// 	return sesssrv, nil
// }
