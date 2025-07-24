package srv

import (
	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/clntcond"
	"sigmaos/sigmasrv/stats"
	"sigmaos/spproto/srv/fid"
	"sigmaos/spproto/srv/leasedmap"
	"sigmaos/spproto/srv/lockmap"
	"sigmaos/spproto/srv/version"
	"sigmaos/spproto/srv/watch"
)

const N = 10000

type ProtSrvState struct {
	plt   *lockmap.PathLockTable
	wt    *watch.WatchTable
	vt    *version.VersionTable
	stats *stats.StatInode
	lm    *leasedmap.LeasedMap
	cct   *clntcond.ClntCondTable
}

func NewProtSrvState(stats *stats.StatInode) *ProtSrvState {
	cct := clntcond.NewClntCondTable()
	pss := &ProtSrvState{
		stats: stats,
		lm:    leasedmap.NewLeasedMap(),
		plt:   lockmap.NewPathLockTable(),
		cct:   cct,
		wt:    watch.NewWatchTable(),
		vt:    version.NewVersionTable(),
	}
	return pss
}

func (pss *ProtSrvState) CondTable() *clntcond.ClntCondTable {
	return pss.cct
}

func (pss *ProtSrvState) VersionTable() *version.VersionTable {
	return pss.vt
}

func (pss *ProtSrvState) PathLockTable() *lockmap.PathLockTable {
	return pss.plt
}

func (pss *ProtSrvState) Leasedmap() *leasedmap.LeasedMap {
	return pss.lm
}

func (pss *ProtSrvState) Stats() *stats.StatInode {
	return pss.stats
}

func (pss *ProtSrvState) newQid(perm sp.Tperm, uid sp.Tuid) sp.Tqid {
	return sp.NewQidPerm(perm, pss.vt.GetVersion(uid), uid.Path)
}

func (pss *ProtSrvState) newFid(fm *fid.FidMap, ctx fs.CtxI, dir fs.Dir, name string, o fs.FsObj, lid sp.TleaseId, qid sp.Tqid) *fid.Fid {
	nf := fm.NewFid(name, o, dir, ctx, 0, qid)
	if o.IsLeased() && pss.lm != nil {
		pss.lm.Insert(o.Path(), lid, name, o, dir)
	}
	return nf
}

// Create name in dir and returns lock for it.
func (pss *ProtSrvState) createObj(ctx fs.CtxI, d fs.Dir, dlk *lockmap.PathLock, name string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence, dev fs.FsObj) (fs.FsObj, *lockmap.PathLock, *serr.Err) {
	if name == "." {
		return nil, nil, serr.NewErr(serr.TErrInval, name)
	}
	// pss.stats.IncPathString(fn.Dir().String())
	o1, err := d.Create(ctx, name, perm, mode, lid, f, dev)
	if err == nil {
		pss.vt.IncVersion(fs.Uid(d))
		pss.wt.AddCreateEvent(dlk.Uid(), name)
		flk := pss.plt.Acquire(ctx, fs.Uid(o1), lockmap.WLOCK)
		return o1, flk, nil
	} else {
		return nil, nil, err
	}
}

func (pss *ProtSrvState) CreateObj(fm *fid.FidMap, ctx fs.CtxI, o fs.FsObj, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, fence sp.Tfence, dev fs.FsObj) (sp.Tqid, *fid.Fid, *serr.Err) {
	db.DPrintf(db.PROTSRV, "%v: Create o %v name %v dev %v", ctx.ClntId(), o, name, dev)
	if !o.Perm().IsDir() {
		return sp.Tqid{}, nil, serr.NewErr(serr.TErrNotDir, name)
	}
	d := o.(fs.Dir)
	dlk := pss.plt.Acquire(ctx, fs.Uid(d), lockmap.WLOCK)
	defer pss.plt.Release(ctx, dlk, lockmap.WLOCK)

	o1, flk, err := pss.createObj(ctx, d, dlk, name, perm, m, lid, fence, dev)
	if lid.IsLeased() {
		db.DPrintf(db.PROTSRV, "%v: createObj Leased %q %v %v lid %v", ctx.ClntId(), name, o1, err, lid)
	}
	if err != nil {
		return sp.Tqid{}, nil, err
	}
	defer pss.plt.Release(ctx, flk, lockmap.WLOCK)

	qid := pss.newQid(o1.Perm(), fs.Uid(o1))
	nf := pss.newFid(fm, ctx, d, name, o1, lid, qid)
	nf.SetMode(m)
	pss.vt.Insert(fs.Uid(o1))
	return qid, nf, nil
}

func (pss *ProtSrvState) OpenObj(ctx fs.CtxI, o fs.FsObj, m sp.Tmode) (fs.FsObj, sp.Tqid, *serr.Err) {
	pss.stats.IncPathString(o.Path().String())
	no, r := o.Open(ctx, m)
	if r != nil {
		return nil, sp.Tqid{}, r
	}
	if no != nil {
		return no, pss.newQid(no.Perm(), fs.Uid(no)), nil
	} else {
		return o, pss.newQid(o.Perm(), fs.Uid(o)), nil
	}
}

func (pss *ProtSrvState) RemoveObj(ctx fs.CtxI, dir fs.Dir, o fs.FsObj, name string, f sp.Tfence, del fs.Tdel) *serr.Err {
	if name == "." {
		return serr.NewErr(serr.TErrInval, name)
	}

	// lock dir to make WatchV and Remove interact correctly
	dlk := pss.plt.Acquire(ctx, fs.Uid(dir), lockmap.WLOCK)
	defer pss.plt.Release(ctx, dlk, lockmap.WLOCK)

	// pss.stats.IncPathString(flk.Path())

	db.DPrintf(db.PROTSRV, "%v: removeObj %v %v", ctx.ClntId(), name, o)

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	leased := o.IsLeased()
	if err := dir.Remove(ctx, name, f, del); err != nil {
		return err
	}

	pss.vt.IncVersion(fs.Uid(dir))
	pss.wt.AddRemoveEvent(dlk.Uid(), name)

	if leased && pss.lm != nil {
		if ok := pss.lm.Delete(o.Path()); !ok {
			// leasesrv may already have removed path from leased
			// map and called RemoveObj to delete it.
			db.DPrintf(db.PROTSRV, "Delete %v doesn't exist in lm", o.Path())
		}
	}
	return nil
}

// Rename this fid.  Other fids for the same underlying fs obj are unchanged.
func (pss *ProtSrvState) RenameObj(f *fid.Fid, name string, fence sp.Tfence) *serr.Err {
	dlk := pss.plt.Acquire(f.Ctx(), f.Uid(), lockmap.WLOCK)
	defer pss.plt.Release(f.Ctx(), dlk, lockmap.WLOCK)

	// pss.stats.IncPathString(po.Pathname().String())

	err := f.Parent().Rename(f.Ctx(), f.Name(), name, fence)
	if err != nil {
		return err
	}

	pss.vt.IncVersion(fs.Uid(f.Parent()))

	pss.wt.AddRemoveEvent(dlk.Uid(), f.Name())
	pss.wt.AddCreateEvent(dlk.Uid(), name)

	if f.Obj().IsLeased() && pss.lm != nil {
		pss.lm.Rename(f.Path(), name)
	}
	f.SetName(name)
	return nil
}

// d1 first?
func lockOrder(d1 fs.FsObj, d2 fs.FsObj) bool {
	if d1.Path() < d2.Path() {
		return true
	} else if d1.Path() == d2.Path() { // would have used wstat instead of renameat
		db.DFatalf("lockOrder")
		return false
	} else {
		return false
	}
}

func (pss *ProtSrvState) RenameAtObj(old, new *fid.Fid, dold, dnew fs.Dir, oldname, newname string, o fs.FsObj, f sp.Tfence) *serr.Err {
	var d1lk, d2lk *lockmap.PathLock
	if srcfirst := lockOrder(dold, dnew); srcfirst {
		d1lk = pss.plt.Acquire(old.Ctx(), fs.Uid(dold), lockmap.WLOCK)
		d2lk = pss.plt.Acquire(new.Ctx(), fs.Uid(dnew), lockmap.WLOCK)
	} else {
		d2lk = pss.plt.Acquire(new.Ctx(), fs.Uid(dnew), lockmap.WLOCK)
		d1lk = pss.plt.Acquire(old.Ctx(), fs.Uid(dold), lockmap.WLOCK)
	}
	defer pss.plt.Release(old.Ctx(), d1lk, lockmap.WLOCK)
	defer pss.plt.Release(new.Ctx(), d2lk, lockmap.WLOCK)

	err := dold.Renameat(old.Ctx(), oldname, dnew, newname, f)
	if err != nil {
		return err
	}

	if o.IsLeased() && pss.lm != nil {
		pss.lm.Rename(o.Path(), newname)
	}

	pss.vt.IncVersion(fs.Uid(new.Obj()))
	pss.vt.IncVersion(fs.Uid(old.Obj()))

	pss.wt.AddRemoveEvent(d1lk.Uid(), oldname)
	pss.wt.AddCreateEvent(d2lk.Uid(), newname)
	return nil
}
