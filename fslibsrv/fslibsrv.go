package fslibsrv

import (
	"sigmaos/proc"
	db "sigmaos/debug"
	"sigmaos/ephemeralmap"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/protsrv"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
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

func BootSrv(scfg *proc.ProcEnv, root fs.Dir, addr string, attachf sps.AttachClntF, detachf sps.DetachClntF, et *ephemeralmap.EphemeralMap) *sesssrv.SessSrv {
	return sesssrv.MakeSessSrv(scfg, root, addr, protsrv.MakeProtServer, attachf, detachf, et, nil)
}

func Post(sesssrv *sesssrv.SessSrv, sc *sigmaclnt.SigmaClnt, path string) error {
	if len(path) > 0 {
		mnt := sp.MkMountServer(sesssrv.MyAddr())
		db.DPrintf(db.BOOT, "Advertise %s at %v\n", path, mnt)
		li, err := sc.LeaseClnt.AskLease(path, fsetcd.LeaseTTL)
		if err != nil {
			return err
		}
		li.KeepExtending()
		if err := sc.MkMountSymlink(path, mnt, li.Lease()); err != nil {
			return err
		}
	}
	return nil
}

func MakeReplServerFsl(root fs.Dir, addr string, path string, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*sesssrv.SessSrv, error) {
	et := ephemeralmap.NewEphemeralMap()
	srv := sesssrv.MakeSessSrv(sc.ProcEnv(), root, addr, protsrv.MakeProtServer, nil, nil, et, fencefs)
	if err := Post(srv, sc, path); err != nil {
		return nil, err
	}
	return srv, nil
}

func MakeSrv(root fs.Dir, path, port string, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*sesssrv.SessSrv, error) {
	return MakeReplServerFsl(root, port, path, sc, fencefs)
}
