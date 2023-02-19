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
	pi   portclnt.PortInfo
}

func MakeMemFs(pn, name string) (*MemFs, error) {
	return MakeMemFsPort(pn, ":0", name)
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

func MakeReplServerPublic(root fs.Dir, path, name string, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, err
	}
	return MakeReplServerClntPublic(root, path, sc, conf, realm)
}

// XXX deduplicate with MakeMemFsPublic
func MakeReplServerClntPublic(root fs.Dir, path string, sc *sigmaclnt.SigmaClnt, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	pc, pi, err := AllocPublicPort(sc)
	if err != nil {
		return nil, err
	}
	srv, err := fslibsrv.MakeReplServerFsl(root, ":"+pi.Pb.RealmPort.String(), "", sc, conf)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	if err = pc.AdvertisePort(path, pi, proc.GetNet(), srv.MyAddr()); err != nil {
		return nil, serr.MkErrError(err)
	}
	return &MemFs{SessSrv: srv, sc: srv.SigmaClnt(), root: root, pc: pc, pi: pi}, nil
}

func MakeReplMemFsFslPublic(addr, path string, sc *sigmaclnt.SigmaClnt, conf repl.Config, realm sp.Trealm) (*MemFs, *serr.Err) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	srv, err := MakeReplServerClntPublic(root, path, sc, conf, realm)
	if err != nil {
		db.DFatalf("Error makeReplMemfsFslPublic: err")
	}
	return srv, nil
}

func MakeReplMemFsFsl(addr, path string, sc *sigmaclnt.SigmaClnt, conf repl.Config) (*MemFs, error) {
	root := dir.MkRootDir(ctx.MkCtx("", 0, nil), memfs.MakeInode)
	return MakeMemFsReplServerFsl(root, addr, path, sc, conf)
}

func AllocPublicPort(sc *sigmaclnt.SigmaClnt) (*portclnt.PortClnt, portclnt.PortInfo, error) {
	pc, err := portclnt.MkPortClnt(sc.FsLib, proc.GetKernelId())
	if err != nil {
		return nil, portclnt.PortInfo{}, err
	}
	pi, err := pc.AllocPort(port.NOPORT)
	if err != nil {
		return nil, portclnt.PortInfo{}, err
	}
	return pc, pi, nil
}

// Allocate server with public port and advertise it
func MakeMemFsPublic(pn, name string) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(name)
	if err != nil {
		return nil, err
	}
	pc, pi, err := AllocPublicPort(sc)
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
