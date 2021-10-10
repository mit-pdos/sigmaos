package fslibsrv

import (
	"fmt"

	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fsclnt"
	"ulambda/fslib"
	fos "ulambda/fsobjsrv"
	"ulambda/fssrv"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/repl"
)

type FsLibSrv struct {
	*fslib.FsLib
	*fssrv.FsServer
}

func (fsl *FsLibSrv) Clnt() *fslib.FsLib {
	return fsl.FsLib
}

func InitFsFsl(name string, fsc *fslib.FsLib, fss *fssrv.FsServer) (*FsLibSrv, error) {
	fsl := &FsLibSrv{fsc, fss}
	err := fsl.PostService(fss.MyAddr(), name)
	if err != nil {
		return nil, fmt.Errorf("PostService %v error: %v", name, err)
	}
	return fsl, nil
}

func InitFs(name string, fss *fssrv.FsServer) (*FsLibSrv, error) {
	fsl := fslib.MakeFsLib(name)
	return InitFsFsl(name, fsl, fss)
}

func makeServer(root fs.Dir, name string) (*fssrv.FsServer, *fslib.FsLib, error) {
	db.Name(name)
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, nil, err
	}
	srv := fssrv.MakeFsServer(root, ip+":0", fos.MakeProtServer(), nil)
	fsl := fslib.MakeFsLib(name)
	return srv, fsl, nil
}

func MakeServer(root fs.Dir, path string, name string) (*fssrv.FsServer, *fslib.FsLib, error) {
	srv, fsl, err := makeServer(root, name)
	if err != nil {
		return nil, nil, err
	}
	if np.EndSlash(path) {
		fsl.Mkdir(path, 0777)
		err = fsl.PostServiceUnion(srv.MyAddr(), path, srv.MyAddr())
		if err != nil {
			return nil, nil, err
		}
	} else {
		err = fsl.PostService(srv.MyAddr(), path)
	}
	if err != nil {
		return nil, nil, err
	}
	return srv, fsl, nil
}

func MakeReplServer(root fs.Dir, addr string, path string, name string, config repl.Config) (*fssrv.FsServer, *fslib.FsLib, error) {
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

type MemFs struct {
	*fslib.FsLib
	*fssrv.FsServer
	root fs.Dir
	ch   chan bool
}

func (fs *MemFs) Root() fs.Dir {
	return fs.root
}

func (fs *MemFs) Wait() {
	<-fs.ch
}

func MakeMemFs(path string, name string) (fs.Dir, *fssrv.FsServer, *fslib.FsLib, error) {
	root := dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	srv, fsl, err := MakeServer(root, path, name)
	return root, srv, fsl, err
}

func StartMemFs(path string, name string) (*MemFs, error) {
	fs := &MemFs{}
	fs.ch = make(chan bool)
	root, srv, fsl, err := MakeMemFs(path, name)
	if err != nil {
		return nil, err
	}
	fs.FsLib = fsl
	fs.FsServer = srv
	fs.root = root

	err = dir.MkNod(fssrv.MkCtx(""), root, "statsd", srv.GetStats())
	if err != nil {
		return nil, err
	}

	go func() {
		srv.Serve()
		fs.ch <- true
	}()
	return fs, err
}
