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
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// Make an MemFs and advertise it at pn
func NewMemFs(pn string, pcfg *proc.ProcEnv) (*MemFs, error) {
	return NewMemFsPort(pn, ":0", pcfg)
}

// Make an MemFs for a specific port and advertise it at pn
func NewMemFsPort(pn, port string, pcfg *proc.ProcEnv) (*MemFs, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.PORT, "NewMemFsPort %v %v\n", pn, port)
	fs, err := NewMemFsPortClnt(pn, port, sc)
	return fs, err
}

// Make an MemFs for a specific port and client, and advertise it at
// pn
func NewMemFsPortClnt(pn, port string, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
	return NewMemFsPortClntFence(pn, port, sc, nil)
}

func NewMemFsPortClntFence(pn, port string, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*MemFs, error) {
	ctx := ctx.NewCtx("", 0, sp.NoClntId, nil, fencefs)
	root := dir.NewRootDir(ctx, memfs.NewInode, nil)
	return NewMemFsRootPortClntFence(root, pn, port, sc, fencefs)
}

func NewMemFsRootPortClntFence(root fs.Dir, pn, port string, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*MemFs, error) {
	srv, err := fslibsrv.NewSrv(root, pn, port, sc, fencefs)
	if err != nil {
		return nil, err
	}
	mfs := NewMemFsSrv(pn, srv, sc, nil)
	return mfs, nil
}

// Allocate server with public port and advertise it
func NewMemFsPublic(pn string, pcfg *proc.ProcEnv) (*MemFs, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	pc, pi, err := portclnt.NewPortClntPort(sc.FsLib)
	if err != nil {
		return nil, err
	}
	// Make server without advertising mnt
	mfs, err := NewMemFsPortClnt("", ":"+pi.Pb.RealmPort.String(), sc)
	if err != nil {
		return nil, err
	}
	mfs.pc = pc

	if err = pc.AdvertisePort(pn, pi, pcfg.GetNet(), mfs.MyAddr()); err != nil {
		return nil, err
	}
	return mfs, err
}
