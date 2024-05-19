// Servers use package memfsssrv to create an in-memory file server.
// memfsssrv uses sesssrv and protsrv to handle client sigmaP
// requests.
package memfssrv

import (
	"sigmaos/auth"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/lockmap"
	"sigmaos/namei"
	"sigmaos/path"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/protsrv"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmapsrv"
)

var rootP = path.Path{""}

const ROOTFID = sp.Tfid(1)

type MemFs struct {
	*sigmapsrv.SigmaPSrv
	ctx  fs.CtxI // server context
	plt  *lockmap.PathLockTable
	ps   *protsrv.ProtSrv
	sc   *sigmaclnt.SigmaClnt
	amgr auth.AuthMgr
	pc   *portclnt.PortClnt
	pi   portclnt.PortInfo
	pn   string
}

func NewMemFsSrv(pn string, srv *sigmapsrv.SigmaPSrv, sc *sigmaclnt.SigmaClnt, amgr auth.AuthMgr, fencefs fs.Dir) *MemFs {
	mfs := &MemFs{
		SigmaPSrv: srv,
		ctx:       ctx.NewCtx(sc.ProcEnv().GetPrincipal(), nil, 0, sp.NoClntId, nil, fencefs),
		plt:       srv.PathLockTable(),
		sc:        sc,
		amgr:      amgr,
		pn:        pn,
		ps:        protsrv.NewProtSrv(srv.ProtSrvState, 0, srv.GetRootCtx),
	}
	return mfs
}

func (mfs *MemFs) SigmaClnt() *sigmaclnt.SigmaClnt {
	return mfs.sc
}

// Note: NewDev() sets parent
func (mfs *MemFs) NewDevInode() *inode.Inode {
	return inode.NewInode(mfs.ctx, sp.DMDEVICE, nil)
}

func (mfs *MemFs) lookup(path path.Path, ltype lockmap.Tlock) (fs.FsObj, *lockmap.PathLock, *serr.Err) {
	d, path := mfs.Root(path)
	lk := mfs.plt.Acquire(mfs.ctx, rootP, ltype)
	if len(path) == 0 {
		return d, lk, nil
	}
	_, lo, lk, _, err := namei.Walk(mfs.plt, mfs.ctx, d, lk, rootP, path, nil, ltype)
	if err != nil {
		mfs.plt.Release(mfs.ctx, lk, ltype)
		return nil, nil, err
	}
	return lo, lk, nil
}

func (mfs *MemFs) lookupParent(path path.Path, ltype lockmap.Tlock) (fs.Dir, *lockmap.PathLock, *serr.Err) {
	lo, lk, err := mfs.lookup(path, ltype)
	if err != nil {
		return nil, nil, err
	}
	d := lo.(fs.Dir)
	return d, lk, nil
}

func (mfs *MemFs) NewDev(pn string, dev fs.FsObj) *serr.Err {
	path, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	d, lk, err := mfs.lookupParent(path.Dir(), lockmap.WLOCK)
	if err != nil {
		return err
	}
	defer mfs.plt.Release(mfs.ctx, lk, lockmap.WLOCK)
	dev.SetParent(d)
	return dir.MkNod(mfs.ctx, d, path.Base(), dev)
}

func (mfs *MemFs) MkNod(pn string, i fs.FsObj) *serr.Err {
	path, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	d, lk, err := mfs.lookupParent(path.Dir(), lockmap.WLOCK)
	if err != nil {
		return err
	}
	defer mfs.plt.Release(mfs.ctx, lk, lockmap.WLOCK)
	return dir.MkNod(mfs.ctx, d, path.Base(), i)
}

func (mfs *MemFs) Create(pn string, p sp.Tperm, m sp.Tmode, lid sp.TleaseId) (fs.FsObj, *serr.Err) {
	path, err := serr.PathSplitErr(pn)
	if err != nil {
		return nil, err
	}
	d, lk, err := mfs.lookupParent(path.Dir(), lockmap.WLOCK)
	if err != nil {
		return nil, err
	}
	defer mfs.plt.Release(mfs.ctx, lk, lockmap.WLOCK)
	return d.Create(mfs.ctx, path.Base(), p, m, lid, sp.NoFence())
}

func (mfs *MemFs) Remove(pn string) *serr.Err {
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	root, rp := mfs.Root(rootP)
	mfs.ps.NewRootFid(ROOTFID, mfs.ctx, root, rp)
	_, _, lo, err := mfs.ps.LookupWalk(ROOTFID, p, false, lockmap.RLOCK)
	if err != nil {
		return err
	}
	return mfs.RemoveObj(mfs.ctx, lo, p, sp.NoFence())
}

func (mfs *MemFs) Open(pn string, m sp.Tmode, ltype lockmap.Tlock) (fs.FsObj, *serr.Err) {
	path, err := serr.PathSplitErr(pn)
	if err != nil {
		return nil, err
	}
	lo, lk, err := mfs.lookup(path, ltype)
	if err != nil {
		return nil, err
	}
	mfs.plt.Release(mfs.ctx, lk, ltype)
	return lo, nil
}

func (mfs *MemFs) MemFsExit(status *proc.Status) error {
	if mfs.pn != "" {
		if err := mfs.sc.Remove(mfs.pn); err != nil {
			db.DPrintf(db.ALWAYS, "RemoveMount %v err %v", mfs.pn, err)
		}
	}
	return mfs.sc.ClntExit(status)
}

func (mfs *MemFs) Dump() error {
	d, path := mfs.Root(rootP)
	s, err := d.(*dir.DirImpl).Dump()
	if err != nil {
		return err
	}
	db.DPrintf("MEMFSSRV", "Dump: %v %v", path, s)
	return nil
}
