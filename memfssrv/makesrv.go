package memfssrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/memfs"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Making single (unreplicated MemFs)
//

// Make an MemFs and advertise it at pn
func MakeMemFs(pn string, uname sp.Tuname) (*MemFs, error) {
	return MakeMemFsPort(pn, ":0", uname)
}

// Make an MemFs for a specific port and advertise it at pn
func MakeMemFsPort(pn, port string, uname sp.Tuname) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(uname)
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
	root := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	srv, err := fslibsrv.MakeSrv(root, pn, port, sc)
	if err != nil {
		return nil, err
	}
	mfs := MakeMemFsSrv(sp.Tuname(pn), pn, srv)
	return mfs, nil
}

// Allocate server with public port and advertise it
func MakeMemFsPublic(pn string, uname sp.Tuname) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(uname)
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
	root := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	return MakeMemFsReplServerFsl(root, addr, path, sc, conf)
}

func MakeMemFsReplServerFsl(root fs.Dir, addr string, path string, sc *sigmaclnt.SigmaClnt, config repl.Config) (*MemFs, error) {
	srv, err := fslibsrv.MakeReplServerFsl(root, addr, path, sc, config)
	if err != nil {
		return nil, err
	}
	return &MemFs{SessSrv: srv, sc: sc}, nil
}

func MakeMemFsReplServer(root fs.Dir, addr, path string, uname sp.Tuname, config repl.Config) (*MemFs, error) {
	srv, err := fslibsrv.MakeReplServer(root, addr, path, uname, config)
	if err != nil {
		return nil, err
	}
	return &MemFs{SessSrv: srv, sc: srv.SigmaClnt()}, nil
}

// This version is for a replicated named, including handling if this
// is the initial named for the root realm.
func MakeReplMemFs(addr, path string, uname sp.Tuname, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	root := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	isInitNamed := false
	// Check if we are one of the initial named replicas
	as, e := proc.Named()
	if e != nil {
		return nil, e
	}
	for _, a := range as {
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
		db.DPrintf(db.PORT, "MakeReplMemFs: not initial one addr %v %v %v %v", addr, path, uname, conf)
		// If this is not the init named, initialize sigma clnt
		if proc.GetNet() == sp.ROOTREALM.String() {
			mfs, err = MakeMemFsReplServer(root, addr, path, uname, conf)
		} else {
			mfs, err = MakeReplServerPublic(root, path, uname, conf, realm)
		}
	}
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	// If this *was* the init named, we now can make sigma clnt
	if isInitNamed {
		sc, err := sigmaclnt.MkSigmaClntFsLib(uname)
		if err != nil {
			return nil, serr.MkErrError(err)
		}
		mfs.sc = sc
		mfs.SetSigmaClnt(sc)
	}
	return mfs, nil
}

// Make replicated memfs with a public port
func MakeReplServerPublic(root fs.Dir, path string, uname sp.Tuname, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	sc, err := sigmaclnt.MkSigmaClnt(uname)
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
	return &MemFs{SessSrv: srv, sc: srv.SigmaClnt(), pc: pc, pi: pi}, nil
}

// Make MemFs with a public port but don't advertise the port (yet)
func MakeReplMemFsFslPublic(path string, sc *sigmaclnt.SigmaClnt, conf repl.Config, realm sp.Trealm) (*MemFs, error) {
	root := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	return MakeReplServerClntPublic(root, "", sc, conf, realm)
}
