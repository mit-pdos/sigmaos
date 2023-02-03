package fslibsrv

import (
	"sigmaos/container"
	"sigmaos/fs"
	"sigmaos/protsrv"
	"sigmaos/repl"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Servers use fslibsrv to make a server and to post their existence
// in the global name space. Servers plug in what a file/directory is
// by passing in their root directory, which is a concrete instance
// for the fs.Dir interface; for example, memfsd passes in an
// in-memory directory, fsux passes in a unix directory etc. This
// allows servers to implement their notions of directories/files, but
// they don't have to implement sigmaP, because fslibsrv provides that
// through sesssrv and protsrv.
//

func makeSrv(root fs.Dir, addr string, sc *sigmaclnt.SigmaClnt, config repl.Config) *sesssrv.SessSrv {
	srv := sesssrv.MakeSessSrv(root, addr, sc, protsrv.MakeProtServer, protsrv.Restore, config)
	return srv
}

func MakeSrv(root fs.Dir, path, port string, sc *sigmaclnt.SigmaClnt) (*sesssrv.SessSrv, error) {
	ip, err := container.LocalIP()
	if err != nil {
		return nil, err
	}
	return MakeReplServerFsl(root, ip+port, path, sc, nil)
}

func MakeReplServerFsl(root fs.Dir, addr string, path string, sc *sigmaclnt.SigmaClnt, config repl.Config) (*sesssrv.SessSrv, error) {
	srv := makeSrv(root, addr, sc, config)
	if len(path) > 0 {
		mnt := sp.MkMountServer(srv.MyAddr())
		err := sc.MkMountSymlink(path, mnt)
		if err != nil {
			return nil, err
		}
	}
	return srv, nil
}

func MakeReplServer(root fs.Dir, addr string, path string, name string, config repl.Config) (*sesssrv.SessSrv, *sigmaclnt.SigmaClnt, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, nil, err
	}
	srv, err := MakeReplServerFsl(root, addr, path, sc, config)
	if err != nil {
		return nil, nil, err
	}
	return srv, sc, nil
}
