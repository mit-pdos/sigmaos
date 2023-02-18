package memfssrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/lockmap"
	"sigmaos/memfs"
	"sigmaos/port"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/serr"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
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
	pc   *portclnt.PortClnt
}

func MakeMemFs(pn, name string) (*MemFs, error) {
	return MakeMemFsPort(pn, ":0", name)
}

func MakeReplMemFs(addr, path, name string, conf repl.Config, realm sp.Trealm) (*sesssrv.SessSrv, *serr.Err) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	isInitNamed := false
	// Check if we are one of the initial named replicas
	for _, a := range proc.Named() {
		if a.Addr == addr {
			isInitNamed = true
			break
		}
	}
	var srv *sesssrv.SessSrv
	var err error
	if isInitNamed {
		srv, err = fslibsrv.MakeReplServerFsl(root, addr, path, nil, conf)
	} else {
		db.DPrintf(db.PORT, "MakeReplMemFs: not initial one addr %v %v %v %v", addr, path, name, conf)
		// If this is not the init named, initialize sigma clnt
		if proc.GetNet() == sp.ROOTREALM.String() {
			srv, err = fslibsrv.MakeReplServer(root, addr, path, name, conf)
		} else {
			srv, err = MakeReplServerPublic(root, addr, path, name, conf, realm)
		}
	}
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	// If this *was* the init named, we now can make sigma clnt
	if isInitNamed {
		sc, err := sigmaclnt.MkSigmaClntFsLib(name)
		if err != nil {
			return nil, serr.MkErrError(err)
		}
		srv.SetSigmaClnt(sc)
	}
	return srv, nil
}

// XXX deduplicate with MakeMemFsPublic
func MakeReplServerPublic(root fs.Dir, addr, path, name string, conf repl.Config, realm sp.Trealm) (*sesssrv.SessSrv, error) {
	sc, pc, hip, pb, err := AllocClntPublicPort(name)
	if err != nil {
		return nil, err
	}

	srv, err := fslibsrv.MakeReplServerFsl(root, ":"+pb.RealmPort.String(), "", sc, conf)
	if err != nil {
		return nil, serr.MkErrError(err)
	}

	if err = pc.AdvertisePort(path, hip, pb, proc.GetNet(), srv.MyAddr()); err != nil {
		return nil, serr.MkErrError(err)
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

func AllocClntPublicPort(name string) (*sigmaclnt.SigmaClnt, *portclnt.PortClnt, string, port.PortBinding, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		db.DFatalf("AllocPublicPort: MakeSigmaClnt err %v", err)
	}
	pc, err := portclnt.MkPortClnt(sc.FsLib, proc.GetKernelId())
	if err != nil {
		return nil, nil, "", port.PortBinding{}, err
	}
	hip, pb, err := pc.AllocPort(port.NOPORT)
	if err != nil {
		return nil, nil, "", port.PortBinding{}, err
	}
	return sc, pc, hip, pb, nil
}

// Allocate server with public port and advertise it
func MakeMemFsPublic(pn, name string) (*MemFs, error) {
	sc, pc, hip, pb, err := AllocClntPublicPort(name)
	if err != nil {
		return nil, err
	}
	// Make server without advertising mnt
	mfs, err := MakeMemFsPortClnt("", ":"+pb.RealmPort.String(), sc)
	if err != nil {
		return nil, err
	}
	mfs.pc = pc

	if err = pc.AdvertisePort(pn, hip, pb, proc.GetNet(), mfs.MyAddr()); err != nil {
		return nil, err
	}

	return mfs, err
}

func MakeMemFsPort(pn, port string, name string) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.PORT, "MakeMemFsPort %v %v\n", pn, port)
	fs, err := MakeMemFsPortClnt(pn, port, sc)
	return fs, err
}

func MakeMemFsPortClnt(pn, port string, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
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
