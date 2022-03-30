package fslibsrv

import (
	"log"

	"ulambda/ctx"
	"ulambda/dir"
	"ulambda/fidclnt"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/procclnt"
	ps "ulambda/protsrv"
	"ulambda/repl"
	"ulambda/sesssrv"
)

func makeSrv(root fs.Dir, addr string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, config repl.Config) *sesssrv.SessSrv {
	srv := sesssrv.MakeSessSrv(root, addr, fsl, ps.MakeProtServer, ps.Restore, pclnt, config)
	return srv
}

func MakeSrv(root fs.Dir, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*sesssrv.SessSrv, error) {
	ip, err := fidclnt.LocalIP()
	if err != nil {
		return nil, err
	}
	return makeReplServerFsl(root, ip+":0", path, fsl, pclnt, nil)
}

func makeReplServerFsl(root fs.Dir, addr string, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, config repl.Config) (*sesssrv.SessSrv, error) {
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
	srv, err := makeReplServerFsl(root, addr, path, fsl, pclnt, config)
	if err != nil {
		return nil, nil, nil, err
	}
	return srv, fsl, pclnt, nil
}

func MakeReplMemFs(addr string, path string, name string, conf repl.Config) (*sesssrv.SessSrv, *np.Err) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	isInitNamed := false
	// Check if we are one of the initial named replicas
	for _, a := range fslib.Named() {
		if a == addr {
			isInitNamed = true
			break
		}
	}
	var srv *sesssrv.SessSrv
	var err error
	// If this is not the init named, initialize the fslib & procclnt
	if !isInitNamed {
		srv, _, _, err = MakeReplServer(root, addr, path, name, conf)
	} else {
		srv, err = makeReplServerFsl(root, addr, path, nil, nil, conf)
	}
	if err != nil {
		return nil, np.MkErrError(err)
	}
	// If this *was* the init named, we now need to init fsl
	if isInitNamed {
		// Server is running, make an fslib for it, mounting itself, to ensure that
		// srv can call checkLock
		srv.SetFsl(fslib.MakeFsLib(name))
	}
	return srv, nil
}

func MakeReplMemFsFsl(addr string, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, conf repl.Config) (*sesssrv.SessSrv, *np.Err) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := makeReplServerFsl(root, addr, path, fsl, pclnt, conf)
	if err != nil {
		log.Fatalf("Error makeReplMemfsFsl: err")
	}
	return srv, nil
}

type MemFs struct {
	*fslib.FsLib
	*sesssrv.SessSrv
	root fs.Dir
}

func (fs *MemFs) Root() fs.Dir {
	return fs.root
}

func MakeMemFs(path string, name string) (*MemFs, *fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl := fslib.MakeFsLib(name)
	pclnt := procclnt.MakeProcClnt(fsl)
	fs, err := MakeMemFsFsl(path, fsl, pclnt)
	return fs, fsl, pclnt, err
}

func MakeMemFsFsl(path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*MemFs, error) {
	fs := &MemFs{}
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := MakeSrv(root, path, fsl, pclnt)
	if err != nil {
		return nil, err
	}
	fs.FsLib = fsl
	fs.SessSrv = srv
	fs.root = root
	return fs, err
}
