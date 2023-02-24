package memfssrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/lockmap"
	"sigmaos/memfs"
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
	pi   portclnt.PortInfo
}

//
// Making single (unreplicated MemFs)
//

// Make an MemFs and advertise it at pn
func MakeMemFs(pn, name string) (*MemFs, error) {
	return MakeMemFsPort(pn, ":0", name)
}

// Make an MemFs for a specific port and advertise it at pn
func MakeMemFsPort(pn, port string, name string) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.PORT, "MakeMemFsPort %v %v\n", pn, port)
	fs, err := MakeMemFsPortClnt(pn, port, sc)
	return fs, err
}

// Make an MemFs for a specific port and client, and advertise it at
// pn
func MakeMemFsPortClnt(pn, port string, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := fslibsrv.MakeSrv(root, pn, port, sc)
	if err != nil {
		return nil, err
	}
	mfs := &MemFs{SessSrv: srv, root: root, ctx: ctx.MkCtx(pn, 0, nil),
		plt: srv.GetPathLockTable(), sc: sc}
	return mfs, nil
}

// Allocate server with public port and advertise it
func MakeMemFsPublic(pn, name string) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, err
	}
	pc, pi, err := portclnt.MkPortClntPort(sc.FsLib)
	if err != nil {
		return nil, err
	}
	// Make server without advertising mnt
	mfs, err := MakeMemFsPortClnt("", ":"+pi.Pb.RealmPort.String(), sc)
	if err != nil {
		return nil, err
	}
	mfs.pc = pc

	if err = pc.AdvertisePort(pn, pi, proc.GetNet(), mfs.MyAddr()); err != nil {
		return nil, err
	}
	return mfs, err
}

//
// Making a MemFs that is part of a replicated memfs service
//

func MakeReplMemFsFsl(addr, path string, sc *sigmaclnt.SigmaClnt, conf repl.Config) (*MemFs, error) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	return MakeMemFsReplServerFsl(root, addr, path, sc, conf)
}

func MakeMemFsReplServerFsl(root fs.Dir, addr string, path string, sc *sigmaclnt.SigmaClnt, config repl.Config) (*MemFs, error) {
	srv, err := fslibsrv.MakeReplServerFsl(root, addr, path, sc, config)
	if err != nil {
		return nil, err
	}
	return &MemFs{SessSrv: srv, root: root, sc: sc}, nil
}

func MakeMemFsReplServer(root fs.Dir, addr string, path, name string, config repl.Config) (*MemFs, error) {
	srv, err := fslibsrv.MakeReplServer(root, addr, path, name, config)
	if err != nil {
		return nil, err
	}
	return &MemFs{SessSrv: srv, root: root, sc: srv.SigmaClnt()}, nil
}

// This version is for a replicated named, including handling if this
// is the initial named for the root realm.
func MakeReplMemFs(addr, path, name string, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	isInitNamed := false
	// Check if we are one of the initial named replicas
	for _, a := range proc.Named() {
		if a.Addr == addr {
			isInitNamed = true
			break
		}
	}
	var mfs *MemFs
	var err error
	if isInitNamed {
		mfs, err = MakeMemFsReplServerFsl(root, addr, path, nil, conf)
	} else {
		db.DPrintf(db.PORT, "MakeReplMemFs: not initial one addr %v %v %v %v", addr, path, name, conf)
		// If this is not the init named, initialize sigma clnt
		if proc.GetNet() == sp.ROOTREALM.String() {
			mfs, err = MakeMemFsReplServer(root, addr, path, name, conf)
		} else {
			mfs, err = MakeReplServerPublic(root, path, name, conf, realm)
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
		mfs.sc = sc
		mfs.SetSigmaClnt(sc)
	}
	return mfs, nil
}

// Make replicated memfs with a public port
func MakeReplServerPublic(root fs.Dir, path, name string, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, err
	}
	return MakeReplServerClntPublic(root, path, sc, conf, realm)
}

// Make MemFs with a public port and advertise the port if valid pn
func MakeReplServerClntPublic(root fs.Dir, path string, sc *sigmaclnt.SigmaClnt, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	pc, pi, err := portclnt.MkPortClntPort(sc.FsLib)
	if err != nil {
		return nil, err
	}
	srv, err := fslibsrv.MakeReplServerFsl(root, ":"+pi.Pb.RealmPort.String(), "", sc, conf)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	if path != "" {
		if err = pc.AdvertisePort(path, pi, proc.GetNet(), srv.MyAddr()); err != nil {
			return nil, serr.MkErrError(err)
		}
	}
	return &MemFs{SessSrv: srv, sc: srv.SigmaClnt(), root: root, pc: pc, pi: pi}, nil
}

// Make MemFs with a public port but don't advertise the port (yet)
func MakeReplMemFsFslPublic(path string, sc *sigmaclnt.SigmaClnt, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	return MakeReplServerClntPublic(root, "", sc, conf, realm)
}
