package memfssrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/fslibsrv"
	"sigmaos/lockmap"
	"sigmaos/memfs"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/serr"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
)

//
// Servers use memfsssrv to create an in-memory file server.
// memfsssrv uses sesssrv and protsrv to handle client sigmaP
// requests.
//

type MemFs struct {
	*sesssrv.SessSrv
	root fs.Dir
	ctx  fs.CtxI // server context
	plt  *lockmap.PathLockTable
	sc   *sigmaclnt.SigmaClnt
}

func MakeReplMemFs(addr string, path string, name string, conf repl.Config) (*sesssrv.SessSrv, *serr.Err) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	isInitNamed := false
	// Check if we are one of the initial named replicas
	for _, a := range proc.Named() {
		if a == addr {
			isInitNamed = true
			break
		}
	}
	var srv *sesssrv.SessSrv
	var err error
	if isInitNamed {
		srv, err = fslibsrv.MakeReplServerFsl(root, addr, path, nil, conf)
	} else {
		// If this is not the init named, initialize the fslib & procclnt
		srv, _, err = fslibsrv.MakeReplServer(root, addr, path, name, conf)
	}
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	// If this *was* the init named, we now need to init fsl
	if isInitNamed {
		// Server is running, make an fslib for it, mounting itself, to ensure that
		// srv can call checkLock
		fsl, err := fslib.MakeFsLib(name)
		if err != nil {
			return nil, serr.MkErrError(err)
		}
		sc, err := sigmaclnt.MkSigmaFsLib(fsl)
		if err != nil {
			return nil, serr.MkErrError(err)
		}
		srv.SetSigmaClnt(sc)
	}
	return srv, nil
}

func MakeReplMemFsFsl(addr string, path string, sc *sigmaclnt.SigmaClnt, conf repl.Config) (*sesssrv.SessSrv, *serr.Err) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := fslibsrv.MakeReplServerFsl(root, addr, path, sc, conf)
	if err != nil {
		db.DFatalf("Error makeReplMemfsFsl: err")
	}
	return srv, nil
}

func MakeMemFs(pn string, name string) (*MemFs, *sigmaclnt.SigmaClnt, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, nil, err
	}
	fs, err := MakeMemFsClnt(pn, sc)
	return fs, sc, err
}

func MakeMemFsClnt(pn string, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
	fs := &MemFs{}
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := fslibsrv.MakeSrv(root, pn, sc)
	if err != nil {
		return nil, err
	}
	fs.SessSrv = srv
	fs.plt = srv.GetPathLockTable()
	fs.sc = sc
	fs.root = root
	fs.ctx = ctx.MkCtx(pn, 0, nil)
	return fs, err
}

func MakeMemFsLib(pn string, fsl *fslib.FsLib) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaFsLib(fsl)
	if err != nil {
		return nil, err
	}
	return MakeMemFsClnt(pn, sc)
}
