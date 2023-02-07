package memfssrv

import (
	"sigmaos/container"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/fslibsrv"
	"sigmaos/lockmap"
	"sigmaos/memfs"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/repl"
	"sigmaos/serr"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

//
// Servers use memfsssrv to create an in-memory file server.
// memfsssrv uses sesssrv and protsrv to handle client sigmaP
// requests.
//

type MemFs struct {
	*sesssrv.SessSrv
	root       fs.Dir
	ctx        fs.CtxI // server context
	plt        *lockmap.PathLockTable
	sc         *sigmaclnt.SigmaClnt
	uprocdclnt *protdevclnt.ProtDevClnt
}

func MakeReplMemFs(addr, path, name string, conf repl.Config) (*sesssrv.SessSrv, *serr.Err) {
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
		sc, err := sigmaclnt.MkSigmaClntFsLib(name)
		if err != nil {
			return nil, serr.MkErrError(err)
		}
		srv.SetSigmaClnt(sc)
	}
	return srv, nil
}

func MakeReplMemFsFsl(addr, path string, sc *sigmaclnt.SigmaClnt, conf repl.Config) (*sesssrv.SessSrv, *serr.Err) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := fslibsrv.MakeReplServerFsl(root, addr, path, sc, conf)
	if err != nil {
		db.DFatalf("Error makeReplMemfsFsl: err")
	}
	return srv, nil
}

func MakeMemFs(pn, name string) (*MemFs, *sigmaclnt.SigmaClnt, error) {
	return MakeMemFsPort(pn, ":0", name)
}

func MakeMemFsPublic(pn, name string) (*MemFs, *sigmaclnt.SigmaClnt, error) {
	realm := proc.GetRealm()

	// mount uprocd
	fsl, err := fslib.MakeFsLibAddr(name, realm, "127.0.0.1", sp.Taddrs{"127.0.0.1:" + container.FPORT.String()})
	pdc, err := protdevclnt.MkProtDevClnt(fsl, "name/")
	if err != nil {
		return nil, nil, err
	}
	req := &proto.PortRequest{
		Realm: realm.String(),
	}
	res := &proto.PortResult{}
	if err := pdc.RPC("UprocSrv.Port", req, res); err != nil {
		return nil, nil, err
	}

	db.DPrintf(db.CACHESRV, "fn %s res.Ip %v res.Port %v\n", pn, res.Ip, res.Port)

	// Make server without advertising mnt
	mfs, sc, err := MakeMemFsPort("", ":"+res.Port, name)
	if err != nil {
		return nil, nil, err
	}

	// Advertise server inside and outside realm
	lip := mfs.MyAddr()
	mnt := sp.MkMountService(sp.Taddrs{lip, res.Ip + ":" + res.Port})

	db.DPrintf(db.CACHESRV, "mnt %v\n", mnt)

	if err := sc.MkMountSymlink(pn, mnt); err != nil {
		return nil, nil, err
	}

	return mfs, sc, err
}

func MakeMemFsPort(pn, port string, name string) (*MemFs, *sigmaclnt.SigmaClnt, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.CACHESRV, "MakeMemFsPort %v %v\n", pn, port)
	fs, err := MakeMemFsSrvClnt(pn, port, sc)
	return fs, sc, err
}

func MakeMemFsSrvClnt(pn, port string, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
	fs := &MemFs{}
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := fslibsrv.MakeSrv(root, pn, port, sc)
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
