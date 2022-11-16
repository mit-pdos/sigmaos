package fslibsrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fidclnt"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/memfs"
	np "sigmaos/ninep"
	"sigmaos/procclnt"
	"sigmaos/protsrv"
	"sigmaos/repl"
	"sigmaos/sesssrv"
)

func makeSrv(root fs.Dir, addr string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, config repl.Config, detach fs.DetachF) *sesssrv.SessSrv {
	srv := sesssrv.MakeSessSrv(root, addr, fsl, protsrv.MakeProtServer, protsrv.Restore, pclnt, config, detach)
	return srv
}

func MakeSrv(root fs.Dir, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, detach fs.DetachF) (*sesssrv.SessSrv, error) {
	ip, err := fidclnt.LocalIP()
	if err != nil {
		return nil, err
	}
	return makeReplServerFsl(root, ip+":0", path, fsl, pclnt, nil, detach)
}

func makeReplServerFsl(root fs.Dir, addr string, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, config repl.Config, detach fs.DetachF) (*sesssrv.SessSrv, error) {
	srv := makeSrv(root, addr, fsl, pclnt, config, detach)
	if len(path) > 0 {
		err := fsl.Post(srv.MyAddr(), path)
		if err != nil {
			return nil, err
		}
	}
	return srv, nil
}

func MakeReplServer(root fs.Dir, addr string, path string, name string, config repl.Config, detach fs.DetachF) (*sesssrv.SessSrv, *fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl := fslib.MakeFsLib(name)
	pclnt := procclnt.MakeProcClnt(fsl)
	srv, err := makeReplServerFsl(root, addr, path, fsl, pclnt, config, detach)
	if err != nil {
		return nil, nil, nil, err
	}
	return srv, fsl, pclnt, nil
}

func MakeReplMemFs(addr string, path string, name string, conf repl.Config, detach fs.DetachF) (*sesssrv.SessSrv, *np.Err) {
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
		srv, _, _, err = MakeReplServer(root, addr, path, name, conf, detach)
	} else {
		srv, err = makeReplServerFsl(root, addr, path, nil, nil, conf, detach)
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

func MakeReplMemFsFsl(addr string, path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, conf repl.Config, detach fs.DetachF) (*sesssrv.SessSrv, *np.Err) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := makeReplServerFsl(root, addr, path, fsl, pclnt, conf, detach)
	if err != nil {
		db.DFatalf("Error makeReplMemfsFsl: err")
	}
	return srv, nil
}

type MemFs struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	*sesssrv.SessSrv
	root fs.Dir
}

func (fs *MemFs) Root() fs.Dir {
	return fs.root
}

func MakeMemFsDetach(path string, name string, detach fs.DetachF) (*MemFs, *fslib.FsLib, *procclnt.ProcClnt, error) {
	fsl := fslib.MakeFsLib(name)
	pclnt := procclnt.MakeProcClnt(fsl)
	fs, err := MakeMemFsFslDetach(path, fsl, pclnt, detach)
	return fs, fsl, pclnt, err
}

func MakeMemFs(path string, name string) (*MemFs, *fslib.FsLib, *procclnt.ProcClnt, error) {
	return MakeMemFsDetach(path, name, nil)
}

func MakeMemFsFslDetach(path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, detach fs.DetachF) (*MemFs, error) {
	fs := &MemFs{}
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := MakeSrv(root, path, fsl, pclnt, detach)
	if err != nil {
		return nil, err
	}
	fs.FsLib = fsl
	fs.ProcClnt = pclnt
	fs.SessSrv = srv
	fs.root = root
	return fs, err
}

func MakeMemFsFsl(path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*MemFs, error) {
	return MakeMemFsFslDetach(path, fsl, pclnt, nil)
}
