package fslibsrv

import (
	"sigmaos/fidclnt"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/procclnt"
	"sigmaos/protsrv"
	"sigmaos/repl"
	"sigmaos/sesssrv"
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

func makeSrv(root fs.Dir, addr string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, config repl.Config) *sesssrv.SessSrv {
	srv := sesssrv.MakeSessSrv(root, addr, fsl, protsrv.MakeProtServer, protsrv.Restore, pclnt, config)
	return srv
}

func MakeSrv(root fs.Dir, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*sesssrv.SessSrv, error) {
	ip, err := fidclnt.LocalIP()
	if err != nil {
		return nil, err
	}
	return MakeReplServerFsl(root, ip+":0", path, fsl, pclnt, nil)
}

func MakeReplServerFsl(root fs.Dir, addr string, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, config repl.Config) (*sesssrv.SessSrv, error) {
	srv := makeSrv(root, addr, fsl, pclnt, config)
	if len(path) > 0 {
		err := fsl.Post(srv.MyAddr(), path)
		if err != nil {
			return nil, err
		}
	}
	return srv, nil
}

func MakeReplServer(root fs.Dir, addr string, path string, name string, config repl.Config) (*sesssrv.SessSrv, *fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl := fslib.MakeFsLib(name)
	pclnt := procclnt.MakeProcClnt(fsl)
	srv, err := MakeReplServerFsl(root, addr, path, fsl, pclnt, config)
	if err != nil {
		return nil, nil, nil, err
	}
	return srv, fsl, pclnt, nil
}
