package fslibsrv

import (
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

func makeSrvClt(root fs.Dir, name string) (*fssrv.FsServer, *fslib.FsLib, error) {
	db.Name(name)
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, nil, err
	}
	srv := fssrv.MakeFsServer(root, ip+":0", fos.MakeProtServer(), nil)
	fsl := fslib.MakeFsLib(name)
	return srv, fsl, nil
}

func MakeSrvClt(root fs.Dir, path string, name string) (*fssrv.FsServer, *fslib.FsLib, error) {
	srv, fsl, err := makeSrvClt(root, name)
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
	if len(path) > 0 {
		fsl.Mkdir(path, 0777)
		err := fsl.PostServiceUnion(srv.MyAddr(), path, srv.MyAddr())
		if err != nil {
			return nil, nil, err
		}
	}
	return srv, fsl, nil
}

func MakeReplMemfs(addr string, path string, name string, conf repl.Config) (*fssrv.FsServer, *fslib.FsLib, error) {
	root := dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	srv, fsl, err := MakeReplServer(root, addr, path, "named", conf)
	if err != nil {
		return nil, nil, err
	}
	err = dir.MkNod(fssrv.MkCtx(""), root, "statsd", srv.GetStats())
	if err != nil {
		return nil, nil, err
	}
	return srv, fsl, err
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

func (fs *MemFs) Wait1() {
	<-fs.ch
}

func MakeMemFs(path string, name string) (fs.Dir, *fssrv.FsServer, *fslib.FsLib, error) {
	root := dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	srv, fsl, err := MakeSrvClt(root, path, name)
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
