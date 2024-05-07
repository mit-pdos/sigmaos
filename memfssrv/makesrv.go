package memfssrv

import (
	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/keys"
	"sigmaos/memfs"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmapsrv"
)

// Make an MemFs and advertise it at pn
func NewMemFs(pn string, pe *proc.ProcEnv) (*MemFs, error) {
	return NewMemFsAddr(pn, sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, sp.NO_PORT, pe.GetNet()), pe)
}

func NewMemFsAddrClnt(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
	db.DPrintf(db.PORT, "NewMemFsPort %v %v\n", pn, addr)
	fs, err := NewMemFsPortClnt(pn, addr, sc)
	return fs, err
}

// Make an MemFs for a specific port and advertise it at pn
func NewMemFsAddr(pn string, addr *sp.Taddr, pe *proc.ProcEnv) (*MemFs, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	return NewMemFsAddrClnt(pn, addr, sc)
}

// Make an MemFs for a specific port and client, and advertise it at
// pn
func NewMemFsPortClnt(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt) (*MemFs, error) {
	return NewMemFsPortClntFence(pn, addr, sc, nil)
}

func NewMemFsPortClntFence(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) (*MemFs, error) {
	return NewMemFsPortClntFenceKeyMgr(pn, addr, sc, fencefs, nil)
}

func NewMemFsPortClntFenceKeyMgr(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir, kmgr auth.KeyMgr) (*MemFs, error) {
	ctx := ctx.NewCtx(sp.NoPrincipal(), nil, 0, sp.NoClntId, nil, fencefs)
	root := dir.NewRootDir(ctx, memfs.NewInode, nil)
	return NewMemFsRootPortClntFenceKeyMgr(root, pn, addr, sc, kmgr, fencefs)
}

func NewMemFsRootPortClntFenceKeyMgr(root fs.Dir, srvpath string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, kmgr auth.KeyMgr, fencefs fs.Dir) (*MemFs, error) {
	// If key mgr is not specified, create a new one
	if kmgr == nil {
		kmgr = keys.NewKeyMgr(keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sc))
	}
	amgr, err := auth.NewAuthMgr[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(sc.ProcEnv().GetPID()), srvpath, kmgr)
	if err != nil {
		db.DPrintf(db.ERROR, "Error new auth srv %v err %v", srvpath, err)
		return nil, err
	}
	srv, mpn, err := sigmapsrv.NewSigmaPSrvPost(root, srvpath, amgr, addr, sc, fencefs)
	if err != nil {
		return nil, err
	}
	mfs := NewMemFsSrv(mpn, srv, sc, amgr, nil)
	return mfs, nil
}

// Allocate server with public port and advertise it
func NewMemFsPublic(pn string, pe *proc.ProcEnv) (*MemFs, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	pc, pi, err := portclnt.NewPortClntPort(sc.FsLib)
	if err != nil {
		return nil, err
	}
	// Make server without advertising ep
	mfs, err := NewMemFsPortClnt("", sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, pi.PBinding.RealmPort, pe.GetNet()), sc)
	if err != nil {
		return nil, err
	}
	mfs.pc = pc

	if err = pc.AdvertisePort(pn, pi, pe.GetNet(), mfs.SigmaPSrv.GetEndpoint()); err != nil {
		return nil, err
	}
	return mfs, err
}
