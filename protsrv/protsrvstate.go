package protsrv

import (
	"sigmaos/clntcond"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/protsrv/ephemeralmap"
	"sigmaos/protsrv/lockmap"
	"sigmaos/protsrv/version"
	"sigmaos/protsrv/watch"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/stats"
)

type ProtSrvState struct {
	plt   *lockmap.PathLockTable
	wt    *watch.WatchTable
	vt    *version.VersionTable
	stats *stats.StatInfo
	et    *ephemeralmap.EphemeralMap
	cct   *clntcond.ClntCondTable
}

func NewProtSrvState(stats *stats.StatInfo) *ProtSrvState {
	cct := clntcond.NewClntCondTable()
	pss := &ProtSrvState{
		stats: stats,
		et:    ephemeralmap.NewEphemeralMap(),
		plt:   lockmap.NewPathLockTable(),
		cct:   cct,
		wt:    watch.NewWatchTable(cct),
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

func (pss *ProtSrvState) EphemeralMap() *ephemeralmap.EphemeralMap {
	return pss.et
}

func (pss *ProtSrvState) Stats() *stats.StatInfo {
	return pss.stats
}

func (pss *ProtSrvState) newQid(perm sp.Tperm, path sp.Tpath) *sp.Tqid {
	return sp.NewQidPerm(perm, pss.vt.GetVersion(path), path)
}

func (pss *ProtSrvState) newFid(ctx fs.CtxI, dir path.Tpathname, name string, o fs.FsObj, lid sp.TleaseId, qid *sp.Tqid) *Fid {
	pn := dir.Copy().Append(name)
	po := newPobj(pn, o, ctx)
	nf := newFidPath(po, 0, qid)
	if o.Perm().IsEphemeral() && pss.et != nil {
		pss.et.Insert(pn.String(), lid)
	}
	return nf
}

// Create name in dir and returns lock for it.
func (pss *ProtSrvState) createObj(ctx fs.CtxI, d fs.Dir, dlk *lockmap.PathLock, fn path.Tpathname, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence, dev fs.FsObj) (fs.FsObj, *lockmap.PathLock, *serr.Err) {
	name := fn.Base()
	if name == "." {
		return nil, nil, serr.NewErr(serr.TErrInval, name)
	}
	pss.stats.IncPathString(fn.Dir().String())
	flk := pss.plt.Acquire(ctx, fn, lockmap.WLOCK)
	o1, err := d.Create(ctx, name, perm, mode, lid, f, dev)
	if perm.IsEphemeral() {
		db.DPrintf(db.PROTSRV, "%v: Create %q %v %v ephemeral %v lid %v", ctx.ClntId(), name, o1, err, perm.IsEphemeral(), lid)
	}
	if err == nil {
		pss.vt.IncVersion(d.Path())
		pss.wt.WakeupWatch(dlk)
		return o1, flk, nil
	} else {
		pss.plt.Release(ctx, flk, lockmap.WLOCK)
		return nil, nil, err
	}
}

func (pss *ProtSrvState) CreateObj(ctx fs.CtxI, o fs.FsObj, dir path.Tpathname, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, fence sp.Tfence, dev fs.FsObj) (*sp.Tqid, *Fid, *serr.Err) {
	db.DPrintf(db.PROTSRV, "%v: Create %v %v", ctx.ClntId(), o, dir)
	fn := dir.Append(name)
	if !o.Perm().IsDir() {
		return nil, nil, serr.NewErr(serr.TErrNotDir, dir)
	}
	d := o.(fs.Dir)
	dlk := pss.plt.Acquire(ctx, dir, lockmap.WLOCK)
	defer pss.plt.Release(ctx, dlk, lockmap.WLOCK)

	o1, flk, err := pss.createObj(ctx, d, dlk, fn, perm, m, lid, fence, dev)
	if err != nil {
		return nil, nil, err
	}
	defer pss.plt.Release(ctx, flk, lockmap.WLOCK)

	pss.vt.Insert(o1.Path())
	qid := pss.newQid(o1.Perm(), o1.Path())
	nf := pss.newFid(ctx, dir, name, o1, lid, qid)
	nf.SetMode(m)

	return qid, nf, nil
}

func (pss *ProtSrvState) OpenObj(ctx fs.CtxI, o fs.FsObj, m sp.Tmode) (fs.FsObj, *sp.Tqid, *serr.Err) {
	pss.stats.IncPathString(o.Path().String())
	no, r := o.Open(ctx, m)
	if r != nil {
		return nil, nil, r
	}
	if no != nil {
		pss.vt.Insert(no.Path())
		pss.vt.IncVersion(no.Path())
		return no, pss.newQid(no.Perm(), no.Path()), nil
	} else {
		return o, pss.newQid(o.Perm(), o.Path()), nil
	}
}

func (pss *ProtSrvState) RemoveObj(ctx fs.CtxI, o fs.FsObj, path path.Tpathname, f sp.Tfence, del fs.Tdel) *serr.Err {
	name := path.Base()
	if name == "." {
		return serr.NewErr(serr.TErrInval, name)
	}

	// lock path to make WatchV and Remove interact correctly
	dlk := pss.plt.Acquire(ctx, path.Dir(), lockmap.WLOCK)
	flk := pss.plt.Acquire(ctx, path, lockmap.WLOCK)
	defer pss.plt.ReleaseLocks(ctx, dlk, flk, lockmap.WLOCK)

	pss.stats.IncPathString(flk.Path())

	db.DPrintf(db.PROTSRV, "%v: removeObj %v %v", ctx.ClntId(), name, o)

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	ephemeral := o.Perm().IsEphemeral()
	if err := o.Parent().Remove(ctx, name, f, del); err != nil {
		return err
	}

	pss.vt.IncVersion(o.Path())
	pss.vt.IncVersion(o.Parent().Path())

	pss.wt.WakeupWatch(dlk)

	if ephemeral && pss.et != nil {
		if ok := pss.et.Delete(path.String()); !ok {
			// leasesrv may already have removed path from ephemeral
			// map and called RemoveObj to delete it.
			db.DPrintf(db.PROTSRV, "Delete %v doesn't exist in et\n", path)
		}
	}
	return nil
}

func (pss *ProtSrvState) RenameObj(po *Pobj, name string, f sp.Tfence) *serr.Err {
	dst := po.Pathname().Dir().Copy().AppendPath(path.Split(name))
	dlk, slk := pss.plt.AcquireLocks(po.Ctx(), po.Pathname().Dir(), po.Pathname().Base(), lockmap.WLOCK)
	defer pss.plt.ReleaseLocks(po.Ctx(), dlk, slk, lockmap.WLOCK)
	tlk := pss.plt.Acquire(po.Ctx(), dst, lockmap.WLOCK)
	defer pss.plt.Release(po.Ctx(), tlk, lockmap.WLOCK)
	pss.stats.IncPathString(po.Pathname().String())
	err := po.Obj().Parent().Rename(po.Ctx(), po.Pathname().Base(), name, f)
	if err != nil {
		return err
	}
	pss.vt.IncVersion(po.Obj().Path())
	pss.vt.IncVersion(po.Obj().Parent().Path())
	pss.wt.WakeupWatch(dlk)
	if po.Obj().Perm().IsEphemeral() && pss.et != nil {
		pss.et.Rename(po.Pathname().String(), dst.String())
	}
	po.SetPath(dst)
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

func (pss *ProtSrvState) RenameAtObj(old, new *Pobj, dold, dnew fs.Dir, oldname, newname string, f sp.Tfence) *serr.Err {
	var d1lk, d2lk, srclk, dstlk *lockmap.PathLock
	if srcfirst := lockOrder(dold, dnew); srcfirst {
		d1lk, srclk = pss.plt.AcquireLocks(old.Ctx(), old.Pathname(), oldname, lockmap.WLOCK)
		d2lk, dstlk = pss.plt.AcquireLocks(new.Ctx(), new.Pathname(), newname, lockmap.WLOCK)
	} else {
		d2lk, dstlk = pss.plt.AcquireLocks(new.Ctx(), new.Pathname(), newname, lockmap.WLOCK)
		d1lk, srclk = pss.plt.AcquireLocks(old.Ctx(), old.Pathname(), oldname, lockmap.WLOCK)
	}
	defer pss.plt.ReleaseLocks(old.Ctx(), d1lk, srclk, lockmap.WLOCK)
	defer pss.plt.ReleaseLocks(new.Ctx(), d2lk, dstlk, lockmap.WLOCK)

	err := dold.Renameat(old.Ctx(), oldname, dnew, newname, f)
	if err != nil {
		return err
	}
	pss.vt.IncVersion(new.Obj().Path())
	pss.vt.IncVersion(old.Obj().Path())

	pss.vt.IncVersion(old.Obj().Parent().Path())
	pss.vt.IncVersion(new.Obj().Parent().Path())

	if old.Obj().Perm().IsEphemeral() && pss.et != nil {
		pss.et.Rename(old.Pathname().String(), new.Pathname().String())
	}

	pss.wt.WakeupWatch(d1lk) // trigger one dir watch
	pss.wt.WakeupWatch(d2lk) // trigger the other dir watch
	return nil
}
