package fslibsrv

import (
	"fmt"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fsclnt"
	"ulambda/fslib"
	fos "ulambda/fsobjsrv"
	"ulambda/fssrv"
	"ulambda/memfsd"
	"ulambda/repl"
)

type FsLibSrv struct {
	*fslib.FsLib
	*memfsd.Fsd
}

func (fsl *FsLibSrv) Clnt() *fslib.FsLib {
	return fsl.FsLib
}

func InitFsFsl(name string, fsc *fslib.FsLib, memfsd *memfsd.Fsd) (*FsLibSrv, error) {
	fsl := &FsLibSrv{fsc, memfsd}
	err := fsl.PostService(memfsd.Addr(), name)
	if err != nil {
		return nil, fmt.Errorf("PostService %v error: %v", name, err)
	}
	return fsl, nil
}

func InitFs(name string, memfsd *memfsd.Fsd) (*FsLibSrv, error) {
	fsl := fslib.MakeFsLib(name)
	return InitFsFsl(name, fsl, memfsd)
}

func (fsl *FsLibSrv) ExitFs(name string) {
	err := fsl.Remove(name)
	if err != nil {
		db.DLPrintf("FSCLNT", "Remove failed %v %v\n", name, err)
	}
}

func MakeSrvFsLib(root fs.Dir, path string, name string) (*fssrv.FsServer, *fslib.FsLib, error) {
	db.Name(name)
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, nil, err
	}
	srv := fssrv.MakeFsServer(root, ip+":0", fos.MakeProtServer(), nil)
	fsl := fslib.MakeFsLib(name)
	fsl.Mkdir(path, 0777)
	err = fsl.PostServiceUnion(srv.MyAddr(), path, srv.MyAddr())
	if err != nil {
		return nil, nil, err
	}
	return srv, fsl, nil
}

func MakeReplSrvFsLib(root fs.Dir, addr string, path string, name string, config repl.Config) (*fssrv.FsServer, *fslib.FsLib, error) {
	db.Name(name)
	srv := fssrv.MakeFsServer(root, addr, fos.MakeProtServer(), config)
	fsl := fslib.MakeFsLib(name)
	fsl.Mkdir(path, 0777)
	err := fsl.PostServiceUnion(srv.MyAddr(), path, srv.MyAddr())
	if err != nil {
		return nil, nil, err
	}
	return srv, fsl, nil
}
