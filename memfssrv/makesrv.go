package memfssrv

import (
	"sigmaos/config"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/memfs"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Make an MemFs and advertise it at pn
func MakeMemFs(pn string, scfg *config.SigmaConfig) (*MemFs, error) {
	return MakeMemFsPort(pn, ":0", scfg)
}

// Make an MemFs for a specific port and advertise it at pn
func MakeMemFsPort(pn, port string, scfg *config.SigmaConfig) (*MemFs, error) {
	sc, err := sigmaclnt.NewSigmaClnt(config.GetSigmaConfig())
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
	return MakeMemFsPortClntFence(pn, port, sc, nil)
}

func MakeMemFsPortClntFence(pn, port string, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*MemFs, error) {
	ctx := ctx.MkCtx("", 0, sp.NoClntId, nil, fencefs)
	root := dir.MkRootDir(ctx, memfs.MakeInode, nil)
	srv, err := fslibsrv.MakeSrv(root, pn, port, sc, fencefs)
	if err != nil {
		return nil, err
	}
	mfs := NewMemFsSrv(pn, srv, sc, nil)
	return mfs, nil
}

// Allocate server with public port and advertise it
func MakeMemFsPublic(pn string, scfg *config.SigmaConfig) (*MemFs, error) {
	sc, err := sigmaclnt.NewSigmaClnt(config.GetSigmaConfig())
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
