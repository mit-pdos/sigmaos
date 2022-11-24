package memfssrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/fslibsrv"
	"sigmaos/memfs"
	np "sigmaos/ninep"
	"sigmaos/procclnt"
	"sigmaos/repl"
	"sigmaos/sesssrv"
)

//
// Servers use memfsssrv to create an in-memory file server.
// memfsssrv uses sesssrv and protsrv to handle client sigmaP
// requests.
//

type MemFs struct {
	*sesssrv.SessSrv
	root     fs.Dir
	ctx      fs.CtxI // server context
	fsl      *fslib.FsLib
	procclnt *procclnt.ProcClnt
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
		srv, _, _, err = fslibsrv.MakeReplServer(root, addr, path, name, conf, detach)
	} else {
		srv, err = fslibsrv.MakeReplServerFsl(root, addr, path, nil, nil, conf, detach)
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
	srv, err := fslibsrv.MakeReplServerFsl(root, addr, path, fsl, pclnt, conf, detach)
	if err != nil {
		db.DFatalf("Error makeReplMemfsFsl: err")
	}
	return srv, nil
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
	srv, err := fslibsrv.MakeSrv(root, path, fsl, pclnt, detach)
	if err != nil {
		return nil, err
	}
	fs.fsl = fsl
	fs.procclnt = pclnt
	fs.SessSrv = srv
	fs.root = root
	fs.ctx = ctx.MkCtx(path, 0, nil)
	return fs, err
}

func MakeMemFsFsl(path string, fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*MemFs, error) {
	return MakeMemFsFslDetach(path, fsl, pclnt, nil)
}
