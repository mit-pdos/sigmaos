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
	"ulambda/procclnt"
	"ulambda/repl"
)

func makeSrv(root fs.Dir, addr string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, config repl.Config) *fssrv.FsServer {
	srv := fssrv.MakeFsServer(root, addr, fsl, fos.MakeProtServer, pclnt, config)
	return srv
}

func MakeSrv(root fs.Dir, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*fssrv.FsServer, error) {
	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, err
	}
	srv := makeSrv(root, ip+":0", fsl, pclnt, nil)
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

func MakeReplServer(root fs.Dir, addr string, path string, name string, config repl.Config) (*fssrv.FsServer, *fslib.FsLib, *procclnt.ProcClnt, error) {
	db.Name(name)
	var fsl *fslib.FsLib
	var pclnt *procclnt.ProcClnt
	isInitNamed := false
	// Check if we are one of the initial named replicas
	for _, a := range fslib.Named() {
		if a == addr {
			isInitNamed = true
			break
		}
	}
	// If this is not the init named, initialize the fslib & procclnt
	if !isInitNamed {
		fsl = fslib.MakeFsLib(name)
		pclnt = procclnt.MakeProcClnt(fsl)
	}
	srv := makeSrv(root, addr, fsl, pclnt, config)
	if len(path) > 0 {
		fsl.Mkdir(path, 0777)
		err := fsl.PostServiceUnion(srv.MyAddr(), path, srv.MyAddr())
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return srv, fsl, pclnt, nil
}

func makeStatDev(root fs.Dir, srv *fssrv.FsServer) error {
	return dir.MkNod(fssrv.MkCtx(""), root, "statsd", srv.GetStats())
}

func MakeReplMemfs(addr string, path string, name string, conf repl.Config) (*fssrv.FsServer, *fslib.FsLib, *procclnt.ProcClnt, error) {
	root := dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode, memfs.GenPath)
	srv, fsl, pclnt, err := MakeReplServer(root, addr, path, "named", conf)
	if err != nil {
		return nil, nil, nil, err
	}
	return srv, fsl, pclnt, makeStatDev(root, srv)
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

func MakeMemFs(path string, name string) (*MemFs, *procclnt.ProcClnt, error) {
	fsl := fslib.MakeFsLib(name)
	pclnt := procclnt.MakeProcClnt(fsl)
	fs, err := MakeMemFsFsl(path, fsl, pclnt)
	return fs, pclnt, err
}

func MakeMemFsFsl(path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*MemFs, error) {
	fs := &MemFs{}
	fs.ch = make(chan bool)
	root := dir.MkRootDir(memfs.MakeInode, memfs.MakeRootInode, memfs.GenPath)
	srv, err := MakeSrv(root, path, fsl, pclnt)
	if err != nil {
		return nil, err
	}
	fs.FsLib = fsl
	fs.FsServer = srv
	fs.root = root
	return fs, makeStatDev(root, srv)
}

func StartMemFsFsl(path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*MemFs, error) {
	fs, err := MakeMemFsFsl(path, fsl, pclnt)
	if err != nil {
		return nil, err
	}

	go func() {
		fs.Serve()
		fs.Done()
	}()
	return fs, err
}

func StartMemFs(path string, name string) (*MemFs, error) {
	fsl := fslib.MakeFsLib(name)
	pclnt := procclnt.MakeProcClnt(fsl)
	return StartMemFsFsl(path, fsl, pclnt)
}
