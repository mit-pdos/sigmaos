package srv

import (
	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/clntcond"
	"sigmaos/sigmasrv/stats"
	"sigmaos/spproto/srv/leasedmap"
	"sigmaos/spproto/srv/lockmapv1"
	"sigmaos/spproto/srv/version"
	"sigmaos/spproto/srv/watch"
)

type ProtSrvState struct {
	plt   *lockmapv1.PathLockTable
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
		plt:   lockmapv1.NewPathLockTable(),
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

func (pss *ProtSrvState) PathLockTable() *lockmapv1.PathLockTable {
	return pss.plt
}

func (pss *ProtSrvState) Leasedmap() *leasedmap.LeasedMap {
	return pss.lm
}

func (pss *ProtSrvState) Stats() *stats.StatInode {
	return pss.stats
}

func (pss *ProtSrvState) newQid(perm sp.Tperm, path sp.Tpath) sp.Tqid {
	return sp.NewQidPerm(perm, pss.vt.GetVersion(path), path)
}

func (pss *ProtSrvState) newFid(ctx fs.CtxI, dir path.Tpathname, name string, o fs.FsObj, lid sp.TleaseId, qid sp.Tqid) *Fid {
	pn := dir.Copy().Append(name)
	po := newPobj(pn, o, ctx)
	nf := newFidPath(po, 0, qid)
	if o.IsLeased() && pss.lm != nil {
		pss.lm.Insert(pn.String(), lid)
	}
	return nf
}

// Create name in dir and returns lock for it.
func (pss *ProtSrvState) createObj(ctx fs.CtxI, d fs.Dir, dlk *lockmapv1.PathLock, name string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence, dev fs.FsObj) (fs.FsObj, *lockmapv1.PathLock, *serr.Err) {
	if name == "." {
		return nil, nil, serr.NewErr(serr.TErrInval, name)
	}
	// pss.stats.IncPathString(fn.Dir().String())
	o1, err := d.Create(ctx, name, perm, mode, lid, f, dev)
	if err == nil {
		pss.vt.IncVersion(d.Path())
		pss.wt.WakeupWatch(dlk)
		flk := pss.plt.Acquire(ctx, o1.Path(), lockmapv1.WLOCK)
		return o1, flk, nil
	} else {
		return nil, nil, err
	}
}

func (pss *ProtSrvState) CreateObj(ctx fs.CtxI, o fs.FsObj, dir path.Tpathname, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, fence sp.Tfence, dev fs.FsObj) (sp.Tqid, *Fid, *serr.Err) {
	db.DPrintf(db.PROTSRV, "%v: Create %v %q", ctx.ClntId(), o, dir)
	if !o.Perm().IsDir() {
		return sp.Tqid{}, nil, serr.NewErr(serr.TErrNotDir, dir)
	}
	d := o.(fs.Dir)
	dlk := pss.plt.Acquire(ctx, d.Path(), lockmapv1.WLOCK)
	defer pss.plt.Release(ctx, dlk, lockmapv1.WLOCK)

	o1, flk, err := pss.createObj(ctx, d, dlk, name, perm, m, lid, fence, dev)
	if lid.IsLeased() {
		db.DPrintf(db.PROTSRV, "%v: createObj Leased %q %v %v lid %v", ctx.ClntId(), name, o1, err, lid)
	}
	if err != nil {
		return sp.Tqid{}, nil, err
	}
	defer pss.plt.Release(ctx, flk, lockmapv1.WLOCK)

	pss.vt.Insert(o1.Path())
	qid := pss.newQid(o1.Perm(), o1.Path())
	nf := pss.newFid(ctx, dir, name, o1, lid, qid)
	nf.SetMode(m)

	return qid, nf, nil
}

func (pss *ProtSrvState) OpenObj(ctx fs.CtxI, o fs.FsObj, m sp.Tmode) (fs.FsObj, sp.Tqid, *serr.Err) {
	pss.stats.IncPathString(o.Path().String())
	no, r := o.Open(ctx, m)
	if r != nil {
		return nil, sp.Tqid{}, r
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

	// lock dir to make WatchV and Remove interact correctly
	dlk := pss.plt.Acquire(ctx, o.Parent().Path(), lockmapv1.WLOCK)
	defer pss.plt.Release(ctx, dlk, lockmapv1.WLOCK)

	// pss.stats.IncPathString(flk.Path())

	db.DPrintf(db.PROTSRV, "%v: removeObj %v %v", ctx.ClntId(), name, o)

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	leased := o.IsLeased()
	if err := o.Parent().Remove(ctx, name, f, del); err != nil {
		return err
	}

	pss.vt.IncVersion(o.Path())
	pss.vt.IncVersion(o.Parent().Path())

	pss.wt.WakeupWatch(dlk)

	if leased && pss.lm != nil {
		if ok := pss.lm.Delete(path.String()); !ok {
			// leasesrv may already have removed path from leased
			// map and called RemoveObj to delete it.
			db.DPrintf(db.PROTSRV, "Delete %v doesn't exist in et\n", path)
		}
	}
	return nil
}

func (pss *ProtSrvState) RenameObj(po *Pobj, name string, f sp.Tfence) *serr.Err {
	dst := po.Pathname().Dir().Copy().AppendPath(path.Split(name))

	dlk := pss.plt.Acquire(po.Ctx(), po.Obj().Path(), lockmapv1.WLOCK)
	defer pss.plt.Release(po.Ctx(), dlk, lockmapv1.WLOCK)

	// pss.stats.IncPathString(po.Pathname().String())

	err := po.Obj().Parent().Rename(po.Ctx(), po.Pathname().Base(), name, f)
	if err != nil {
		return err
	}
	pss.vt.IncVersion(po.Obj().Path())

	// XXX update version name
	// pss.vt.IncVersion(po.Obj().Parent().Path())

	pss.wt.WakeupWatch(dlk)
	if po.Obj().IsLeased() && pss.lm != nil {
		pss.lm.Rename(po.Pathname().String(), dst.String())
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
	var d1lk, d2lk *lockmapv1.PathLock
	if srcfirst := lockOrder(dold, dnew); srcfirst {
		d1lk = pss.plt.Acquire(old.Ctx(), dold.Path(), lockmapv1.WLOCK)
		d2lk = pss.plt.Acquire(new.Ctx(), dnew.Path(), lockmapv1.WLOCK)
	} else {
		d2lk = pss.plt.Acquire(new.Ctx(), dnew.Path(), lockmapv1.WLOCK)
		d1lk = pss.plt.Acquire(old.Ctx(), dold.Path(), lockmapv1.WLOCK)
	}
	defer pss.plt.Release(old.Ctx(), d1lk, lockmapv1.WLOCK)
	defer pss.plt.Release(new.Ctx(), d2lk, lockmapv1.WLOCK)

	err := dold.Renameat(old.Ctx(), oldname, dnew, newname, f)
	if err != nil {
		return err
	}

	pss.vt.IncVersion(new.Obj().Path())
	pss.vt.IncVersion(old.Obj().Path())

	// XXX Update files versions
	// pss.vt.IncVersion(old.Obj().Parent().Path())
	// pss.vt.IncVersion(new.Obj().Parent().Path())

	if old.Obj().IsLeased() && pss.lm != nil {
		pss.lm.Rename(old.Pathname().String(), new.Pathname().String())
	}

	pss.wt.WakeupWatch(d1lk) // trigger one dir watch
	pss.wt.WakeupWatch(d2lk) // trigger the other dir watch
	return nil
}
