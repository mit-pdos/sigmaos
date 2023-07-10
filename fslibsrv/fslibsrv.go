package fslibsrv

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/protsrv"
	"sigmaos/repl"
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

func BootSrv(root fs.Dir, addr string, sc *sigmaclnt.SigmaClnt, attachf sps.AttachClntF, detachf sps.DetachClntF, config repl.Config) *sesssrv.SessSrv {
	return sesssrv.MakeSessSrv(root, addr, sc, protsrv.MakeProtServer, protsrv.Restore, config, attachf, detachf)
}

func MakeReplServerFsl(root fs.Dir, addr string, path string, sc *sigmaclnt.SigmaClnt, config repl.Config) (*sesssrv.SessSrv, error) {
	srv := sesssrv.MakeSessSrv(root, addr, sc, protsrv.MakeProtServer, protsrv.Restore, config, nil, nil)
	if len(path) > 0 {
		mnt := sp.MkMountServer(srv.MyAddr())
		db.DPrintf(db.BOOT, "Advertise %s at %v\n", path, mnt)
		lid, err := sc.LeaseMgrClnt.AskLease(path, fsetcd.LeaseTTL)
		if err != nil {
			return nil, err
		}
		sc.LeaseMgrClnt.KeepExtending(lid)
		if err := sc.MkMountSymlink(path, mnt, lid); err != nil {
			return nil, err
		}
	}
	return srv, nil
}
func MakeSrv(root fs.Dir, path, port string, sc *sigmaclnt.SigmaClnt) (*sesssrv.SessSrv, error) {
	return MakeReplServerFsl(root, port, path, sc, nil)
}

func MakeReplServer(root fs.Dir, addr string, path string, name sp.Tuname, config repl.Config) (*sesssrv.SessSrv, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, err
	}
	srv, err := MakeReplServerFsl(root, addr, path, sc, config)
	if err != nil {
		return nil, err
	}
	return srv, nil
}
