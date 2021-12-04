package fslibsrv

import (
	"log"

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

func makeSrv(root fs.Dir, addr string, fsl *fslib.FsLib, config repl.Config) *fssrv.FsServer {
	srv := fssrv.MakeFsServer(root, addr, fsl, fos.MakeProtServer(), config)
	return srv
}

func MakeSrv(root fs.Dir, path string, fsl *fslib.FsLib) (*fssrv.FsServer, error) {
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, err
	}
	srv := makeSrv(root, ip+":0", fsl, nil)
	if np.EndSlash(path) {
		fsl.Mkdir(path, 0777)
		err = fsl.PostServiceUnion(srv.MyAddr(), path, srv.MyAddr())
	} else {
		err = fsl.PostService(srv.MyAddr(), path)
	}
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func MakeReplServer(root fs.Dir, addr string, path string, name string, config repl.Config) (*fssrv.FsServer, *fslib.FsLib, error) {
	db.Name(name)
	log.Printf("MakeReplServer: %v\n", name)
	fsl := fslib.MakeFsLib(name)
	srv := makeSrv(root, addr, fsl, config)
	if len(path) > 0 {
		fsl.Mkdir(path, 0777)
		err := fsl.PostServiceUnion(srv.MyAddr(), path, srv.MyAddr())
		if err != nil {
			return nil, nil, err
		}
	}
	return srv, fsl, nil
}

func makeStatDev(root fs.Dir, srv *fssrv.FsServer) error {
	return dir.MkNod(fssrv.MkCtx(""), root, "statsd", srv.GetStats())
}

func MakeReplMemfs(addr string, path string, name string, conf repl.Config) (*fssrv.FsServer, *fslib.FsLib, error) {
	root := dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	srv, fsl, err := MakeReplServer(root, addr, path, "named", conf)
	if err != nil {
		return nil, nil, err
	}
	err = fsl.MountTree(fslib.Named(), "", "name")
	if err != nil {
		log.Fatalf("%v: Mount %v error: %v", db.GetName(), fslib.Named(), err)
	}
	return srv, fsl, makeStatDev(root, srv)
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
	fsl := fslib.MakeFsLib(name)
	srv, err := MakeSrv(root, path, fsl)
	if err != nil {
		return nil, nil, nil, err
	}
	return root, srv, fsl, makeStatDev(root, srv)
}

func MakeMemFsFsl(path string, fsl *fslib.FsLib) (fs.Dir, *fssrv.FsServer, error) {
	root := dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode)
	srv, err := MakeSrv(root, path, fsl)
	if err != nil {
		return nil, nil, err
	}
	return root, srv, makeStatDev(root, srv)
}

func StartMemFsFsl(path string, fsl *fslib.FsLib) (*MemFs, error) {
	fs := &MemFs{}
	fs.ch = make(chan bool)
	root, srv, err := MakeMemFsFsl(path, fsl)
	if err != nil {
		return nil, err
	}
	fs.FsLib = fsl
	fs.FsServer = srv
	fs.root = root

	go func() {
		srv.Serve()
		fs.ch <- true
	}()
	return fs, err
}

func StartMemFs(path string, name string) (*MemFs, error) {
	fsl := fslib.MakeFsLib(name)
	return StartMemFsFsl(path, fsl)
}
