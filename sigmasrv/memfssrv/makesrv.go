package memfssrv

import (
	"time"

	"sigmaos/api/fs"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/proc"
	sessrv "sigmaos/session/srv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs"
	"sigmaos/sigmasrv/memfssrv/memfs/dir"
	"sigmaos/sigmasrv/memfssrv/memfs/fenceddir"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
	"sigmaos/sigmasrv/memfssrv/sigmapsrv"
	spprotosrv "sigmaos/spproto/srv"
	"sigmaos/util/perf"
)

// Make an MemFs and advertise it at pn
func NewMemFs(pn string, pe *proc.ProcEnv, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	return NewMemFsAddr(pn, sp.NewTaddr(sp.NO_IP, sp.NO_PORT), pe, aaf)
}

func NewMemFsAddrClnt(pn string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	fs, err := NewMemFsPortClnt(pn, addr, sc, aaf)
	return fs, err
}

// Make an MemFs for a specific port and advertise it at pn
func NewMemFsAddr(pn string, addr *sp.Taddr, pe *proc.ProcEnv, aaf spprotosrv.AttachAuthF) (*MemFs, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	start := time.Now()
	defer func() {
		perf.LogSpawnLatency("NewMemFsAddr.NewMemFsAddrClnt", pe.GetPID(), pe.GetSpawnTime(), start)
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
	ni := memfs.NewNewInode(sp.DEV_MEMFS)
	root := fenceddir.NewFencedRoot(dir.NewRootDir(ctx, ni))
	return NewMemFsRootPortClntFenceAuthExp(root, pn, addr, sc, fencefs, aaf, ni.InodeAlloc(), nil)
}

func NewMemFsRootPortClntFenceAuthExp(root fs.Dir, srvpath string, addr *sp.Taddr, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir, aaf spprotosrv.AttachAuthF, ia *inode.InodeAlloc, exp sessrv.ExpireI) (*MemFs, error) {
	start := time.Now()
	srv, mpn, err := sigmapsrv.NewSigmaPSrvPost(root, srvpath, addr, sc, fencefs, aaf, exp)
	if err != nil {
		return nil, err
	}
	perf.LogSpawnLatency("NewMemFsAddr.NewSigmaPSrvPost", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	start = time.Now()
	defer func() {
		perf.LogSpawnLatency("NewMemFsAddr.NewMemFsSrv", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	}()
	mfs := NewMemFsSrv(mpn, srv, sc, nil, ia)
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
