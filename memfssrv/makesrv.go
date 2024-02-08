package memfssrv

import (
	"sigmaos/auth"
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
	return NewMemFsAddr(pn, sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, sp.NO_PORT, pcfg.GetNet()), pcfg)
}

// Make an MemFs for a specific port and advertise it at pn
func NewMemFsAddr(pn string, addr *sp.Taddr, pcfg *proc.ProcEnv) (*MemFs, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.PORT, "NewMemFsPort %v %v\n", pn, addr)
	fs, err := NewMemFsPortClnt(pn, addr, sc)
	return fs, err
}

// Make an MemFs for a specific port and client, and advertise it at
// pn
func NewMemFsPortClnt(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
	return NewMemFsPortClntFence(pn, addr, sc, nil)
}

func NewMemFsPortClntFence(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*MemFs, error) {
	ctx := ctx.NewCtx(sp.NoPrincipal(), nil, 0, sp.NoClntId, nil, fencefs)
	root := dir.NewRootDir(ctx, memfs.NewInode, nil)
	return NewMemFsRootPortClntFenceKey(root, pn, addr, sc, nil, fencefs)
}

func NewMemFsRootPortClntFenceKey(root fs.Dir, pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, key auth.SymmetricKey, fencefs fs.Dir) (*MemFs, error) {
	var as auth.AuthSrv
	var err error
	if key == nil {
		as, err = NewHMACVerificationSrv(sp.Tsigner(sc.ProcEnv().GetPID()), pn, sc)
		if err != nil {
			return nil, err
		}
	} else {
		as, err = NewHMACVerificationSrvKey(sp.Tsigner(sc.ProcEnv().GetPID()), pn, sc, key)
		if err != nil {
			return nil, err
		}
	}
	srv, mpn, err := fslibsrv.NewSrv(root, pn, as, addr, sc, fencefs)
	if err != nil {
		return nil, err
	}
	mfs := NewMemFsSrv(mpn, srv, sc, as, nil)
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
	mfs, err := NewMemFsPortClnt("", sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, pi.PBinding.RealmPort, pcfg.GetNet()), sc)
	if err != nil {
		return nil, err
	}
	mfs.pc = pc

	if err = pc.AdvertisePort(pn, pi, pcfg.GetNet(), mfs.MyAddr()); err != nil {
		return nil, err
	}
	return mfs, err
}
