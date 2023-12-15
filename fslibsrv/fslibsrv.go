package fslibsrv

import (
	db "sigmaos/debug"
	"sigmaos/ephemeralmap"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/protsrv"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Servers use fslibsrv to make a server and to post their existence
// in the global name space. Servers plug in what a file/directory is
// by passing in their root directory, which is a concrete instance of
// the fs.Dir interface; for example, memfsd passes in an in-memory
// directory, fsux passes in a unix directory etc. This allows servers
// to implement their notions of directories/files, but they don't
// have to implement sigmaP, because fslibsrv provides that through
// sesssrv and protsrv.
//

func Post(sesssrv *sesssrv.SessSrv, sc *sigmaclnt.SigmaClnt, path string) error {
	if len(path) > 0 {
		mnt := sp.NewMountServer(sesssrv.MyAddr())
		db.DPrintf(db.BOOT, "Advertise %s at %v\n", path, mnt)
		li, err := sc.LeaseClnt.AskLease(path, fsetcd.LeaseTTL)
		if err != nil {
			return err
		}
		li.KeepExtending()
		if err := sc.NewMountSymlink(path, mnt, li.Lease()); err != nil {
			return err
		}
	}
	return nil
}

func NewSrv(root fs.Dir, path, port string, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*sesssrv.SessSrv, error) {
	et := ephemeralmap.NewEphemeralMap()
	srv := sesssrv.NewSessSrv(sc.ProcEnv(), root, port, protsrv.NewProtServer, nil, nil, et, fencefs)
	if err := Post(srv, sc, path); err != nil {
		return nil, err
	}
	return srv, nil
}
