package memfssrv

import (
	"time"

	"sigmaos/api/fs"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs"
	"sigmaos/sigmasrv/memfssrv/memfs/dir"
	"sigmaos/sigmasrv/memfssrv/memfs/fenceddir"
	"sigmaos/sigmasrv/memfssrv/sigmapsrv"
	spprotosrv "sigmaos/spproto/srv"
)

// Make an MemFs and advertise it at pn
func NewMemFs(pn string, pe *proc.ProcEnv, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	return NewMemFsAddr(pn, sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, sp.NO_PORT), pe, aaf)
}

func NewMemFsAddrClnt(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	fs, err := NewMemFsPortClnt(pn, addr, sc, aaf)
	return fs, err
}

// Make an MemFs for a specific port and advertise it at pn
func NewMemFsAddr(pn string, addr *sp.Taddr, pe *proc.ProcEnv, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	start := time.Now()
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "NewMemFsAddr NewSigmaClnt: %v", time.Since(start))
	start = time.Now()
	defer func() {
		db.DPrintf(db.SPAWN_LAT, "NewMemFsAddr NewMemFsAddrClnt: %v", time.Since(start))
	}()
	return NewMemFsAddrClnt(pn, addr, sc, aaf)
}

// Make an MemFs for a specific port and client, and advertise it at
// pn
func NewMemFsPortClnt(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	return NewMemFsPortClntFenceAuth(pn, addr, sc, nil, aaf)
}

func NewMemFsPortClntFenceAuth(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	ctx := ctx.NewCtx(sp.NoPrincipal(), nil, 0, sp.NoClntId, nil, fencefs)
	root := fenceddir.NewFencedRoot(dir.NewRootDir(ctx, memfs.NewInode))
	return NewMemFsRootPortClntFenceAuth(root, pn, addr, sc, fencefs, aaf)
}

func NewMemFsRootPortClntFenceAuth(root fs.Dir, srvpath string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	start := time.Now()
	srv, mpn, err := sigmapsrv.NewSigmaPSrvPost(root, srvpath, addr, sc, fencefs, aaf)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "NewMemFsRootPortClntFenceAuth NewSigmaPSrvPost: %v", time.Since(start))
	start = time.Now()
	defer func() {
		db.DPrintf(db.SPAWN_LAT, "NewMemFsRootPortClntFenceAuth NewMemFsSrv: %v", time.Since(start))
	}()
	mfs := NewMemFsSrv(mpn, srv, sc, nil)
	return mfs, nil
}

func (mfs *MemFs) MemFsExit(status *proc.Status) error {
	if mfs.pn != "" {
		if err := mfs.sc.Remove(mfs.pn); err != nil {
			db.DPrintf(db.ALWAYS, "RemoveMount %v err %v", mfs.pn, err)
		}
	}
	return mfs.sc.ClntExit(status)
}
