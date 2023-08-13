package protsrv

import (
	db "sigmaos/debug"
	"sigmaos/ephemeralmap"
	"sigmaos/fid"
	"sigmaos/fs"
	"sigmaos/lockmap"
	"sigmaos/namei"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/sesssrv"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
	"sigmaos/stats"
	"sigmaos/version"
	"sigmaos/watch"
)

//
// There is one protsrv per session, but they share the watch table,
// version table, and stats across sessions.  Each session has its own
// fid table, ephemeral table, and lease table.
//

type ProtSrv struct {
	ssrv  *sesssrv.SessSrv
	plt   *lockmap.PathLockTable     // shared across sessions
	wt    *watch.WatchTable          // shared across sessions
	vt    *version.VersionTable      // shared across sessions
	stats *stats.StatInfo            // shared across sessions
	et    *ephemeralmap.EphemeralMap // shared across sessions
	ft    *fidTable
	sid   sessp.Tsession
}

func MakeProtServer(s sps.SessServer, sid sessp.Tsession) sps.Protsrv {
	ps := &ProtSrv{}
	srv := s.(*sesssrv.SessSrv)
	ps.ssrv = srv

	ps.ft = makeFidTable()
	ps.et = srv.GetEphemeralMap()
	ps.plt = srv.GetPathLockTable()
	ps.wt = srv.GetWatchTable()
	ps.vt = srv.GetVersionTable()
	ps.stats = srv.GetStats()
	ps.sid = sid
	db.DPrintf(db.PROTSRV, "MakeProtSrv -> %v", ps)
	return ps
}

func (ps *ProtSrv) mkQid(perm sp.Tperm, path sp.Tpath) *sp.Tqid {
	return sp.MakeQidPerm(perm, ps.vt.GetVersion(path), path)
}

func (ps *ProtSrv) Version(args *sp.Tversion, rets *sp.Rversion) *sp.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (ps *ProtSrv) Auth(args *sp.Tauth, rets *sp.Rauth) *sp.Rerror {
	return sp.MkRerror(serr.MkErr(serr.TErrNotSupported, "Auth"))
}

func (ps *ProtSrv) Attach(args *sp.Tattach, rets *sp.Rattach, attach sps.AttachClntF) *sp.Rerror {
	db.DPrintf(db.PROTSRV, "Attach %v sid %v", args, ps.sid)
	p := path.Split(args.Aname)
	root, ctx := ps.ssrv.GetRootCtx(args.Tuname(), args.Aname, ps.sid, args.TclntId())
	tree := root.(fs.FsObj)
	qid := ps.mkQid(tree.Perm(), tree.Path())
	if args.Aname != "" {
		dlk := ps.plt.Acquire(ctx, path.Path{})
		_, lo, lk, rest, err := namei.Walk(ps.plt, ctx, root, dlk, path.Path{}, p, nil)
		defer ps.plt.Release(ctx, lk)
		if len(rest) > 0 || err != nil {
			return sp.MkRerror(err)
		}
		// insert before releasing
		ps.vt.Insert(lo.Path())
		tree = lo
		qid = ps.mkQid(lo.Perm(), lo.Path())
	} else {
		// root is already in the version table; this updates
		// just the refcnt.
		ps.vt.Insert(root.Path())
	}
	ps.ft.Add(args.Tfid(), fid.MakeFidPath(fid.MkPobj(p, tree, ctx), 0, qid))
	rets.Qid = qid
	if attach != nil {
		attach(args.TclntId())
	}
	return nil
}

// Delete ephemeral files created on this session.
func (ps *ProtSrv) Detach(args *sp.Tdetach, rets *sp.Rdetach, detach sps.DetachClntF) *sp.Rerror {
	db.DPrintf(db.PROTSRV, "Detach cid %v sess %v\n", args.TclntId(), ps.sid)

	// Several threads maybe waiting in a sesscond. DeleteSess
	// will unblock them so that they can bail out.
	ps.ssrv.GetSessCondTable().DeleteSess(ps.sid)

	ps.ft.ClunkOpen()
	if detach != nil {
		detach(args.TclntId())
	}
	return nil
}

func (ps *ProtSrv) makeQids(os []fs.FsObj) []*sp.Tqid {
	var qids []*sp.Tqid
	for _, o := range os {
		qids = append(qids, ps.mkQid(o.Perm(), o.Path()))
	}
	return qids
}

func (ps *ProtSrv) lookupObjLast(ctx fs.CtxI, f *fid.Fid, names path.Path, resolve bool) (fs.FsObj, *serr.Err) {
	_, lo, lk, _, err := ps.lookupObj(ctx, f.Pobj(), names)
	ps.plt.Release(ctx, lk)
	if err != nil {
		return nil, err
	}
	if lo.Perm().IsSymlink() && resolve {
		return nil, serr.MkErr(serr.TErrNotDir, names[len(names)-1])
	}
	return lo, nil
}

// Requests that combine walk, open, and do operation in a single RPC,
// which also avoids clunking. They may fail because args.Wnames may
// contains a special path element; in that, case the client must walk
// args.Wnames.
func (ps *ProtSrv) Walk(args *sp.Twalk, rets *sp.Rwalk) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}

	db.DPrintf(db.PROTSRV, "%v: Walk o %v args {%v} (%v)", f.Pobj().Ctx().Uname(), f, args, len(args.Wnames))

	os, lo, lk, rest, err := ps.lookupObj(f.Pobj().Ctx(), f.Pobj(), args.Wnames)
	defer ps.plt.Release(f.Pobj().Ctx(), lk)

	if lk != nil {
		ps.stats.IncPathString(lk.Path())
	}

	if err != nil && !err.IsMaybeSpecialElem() {
		return sp.MkRerror(err)
	}

	// let the client decide what to do with rest (when there is a rest)
	n := len(args.Wnames) - len(rest)
	p := append(f.Pobj().Path().Copy(), args.Wnames[:n]...)
	rets.Qids = ps.makeQids(os)
	qid := ps.mkQid(lo.Perm(), lo.Path())
	db.DPrintf(db.PROTSRV, "%v: Walk MakeFidPath fid %v p %v lo %v qid %v os %v", args.NewFid, f.Pobj().Ctx().Uname(), p, lo, qid, os)
	ps.ft.Add(args.Tnewfid(), fid.MakeFidPath(fid.MkPobj(p, lo, f.Pobj().Ctx()), 0, qid))

	ps.vt.Insert(qid.Tpath())

	return nil
}

func (ps *ProtSrv) Clunk(args *sp.Tclunk, rets *sp.Rclunk) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Clunk %v f %v path %v", f.Pobj().Ctx().Uname(), args.Fid, f, f.Pobj().Path())
	if f.IsOpen() { // has the fid been opened?
		f.Pobj().Obj().Close(f.Pobj().Ctx(), f.Mode())
		f.Close()
	}
	ps.ft.Del(args.Tfid())
	ps.vt.Delete(f.Pobj().Obj().Path())
	return nil
}

func (ps *ProtSrv) Open(args *sp.Topen, rets *sp.Ropen) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Open f %v %v", f.Pobj().Ctx().Uname(), f, args)

	ps.stats.IncPathString(f.Pobj().Path().String())

	o := f.Pobj().Obj()
	no, r := o.Open(f.Pobj().Ctx(), args.Tmode())
	if r != nil {
		return sp.MkRerror(r)
	}
	f.SetMode(args.Tmode())
	if no != nil {
		f.Pobj().SetObj(no)
		ps.vt.Insert(no.Path())
		ps.vt.IncVersion(no.Path())
		rets.Qid = ps.mkQid(no.Perm(), no.Path())
	} else {
		rets.Qid = ps.mkQid(o.Perm(), o.Path())
	}
	return nil
}

func (ps *ProtSrv) Watch(args *sp.Twatch, rets *sp.Ropen) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	p := f.Pobj().Path()
	ino := f.Pobj().Obj().Path()

	db.DPrintf(db.PROTSRV, "%v: Watch %v v %v %v", f.Pobj().Ctx().Uname(), f.Pobj().Path(), f.Qid(), args)

	// get path lock on for p, so that remove cannot remove file
	// before watch is set.
	pl := ps.plt.Acquire(f.Pobj().Ctx(), p)
	defer ps.plt.Release(f.Pobj().Ctx(), pl)

	v := ps.vt.GetVersion(ino)
	if !sp.VEq(f.Qid().Tversion(), v) {
		return sp.MkRerror(serr.MkErr(serr.TErrVersion, v))
	}
	err = ps.wt.WaitWatch(pl, ps.sid)
	if err != nil {
		return sp.MkRerror(err)
	}
	return nil
}

func (ps *ProtSrv) makeFid(ctx fs.CtxI, dir path.Path, name string, o fs.FsObj, lid sp.TleaseId, qid *sp.Tqid) *fid.Fid {
	pn := dir.Copy().Append(name)
	po := fid.MkPobj(pn, o, ctx)
	nf := fid.MakeFidPath(po, 0, qid)
	if o.Perm().IsEphemeral() && ps.et != nil {
		ps.et.Insert(pn.String(), lid)
	}
	return nf
}

// Create name in dir and returns lock for it.
func (ps *ProtSrv) createObj(ctx fs.CtxI, d fs.Dir, dlk *lockmap.PathLock, fn path.Path, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (fs.FsObj, *lockmap.PathLock, *serr.Err) {
	name := fn.Base()
	if name == "." {
		return nil, nil, serr.MkErr(serr.TErrInval, name)
	}
	flk := ps.plt.Acquire(ctx, fn)
	o1, err := d.Create(ctx, name, perm, mode, lid, f)
	db.DPrintf(db.PROTSRV, "%v: Create %q %v %v ephemeral %v %v lid %v", ctx.Uname(), name, o1, err, perm.IsEphemeral(), ps.sid, lid)
	if err == nil {
		ps.wt.WakeupWatch(dlk)
		return o1, flk, nil
	} else {
		ps.plt.Release(ctx, flk)
		return nil, nil, err
	}
}

func (ps *ProtSrv) Create(args *sp.Tcreate, rets *sp.Rcreate) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Create f %v", f.Pobj().Ctx().Uname(), f)
	o := f.Pobj().Obj()
	fn := f.Pobj().Path().Append(args.Name)
	if !o.Perm().IsDir() {
		return sp.MkRerror(serr.MkErr(serr.TErrNotDir, f.Pobj().Path()))
	}
	d := o.(fs.Dir)
	dlk := ps.plt.Acquire(f.Pobj().Ctx(), f.Pobj().Path())
	defer ps.plt.Release(f.Pobj().Ctx(), dlk)

	o1, flk, err := ps.createObj(f.Pobj().Ctx(), d, dlk, fn, args.Tperm(), args.Tmode(), args.TleaseId(), args.Tfence())
	if err != nil {
		return sp.MkRerror(err)
	}
	defer ps.plt.Release(f.Pobj().Ctx(), flk)
	ps.stats.IncPathString(f.Pobj().Path().String())
	ps.vt.Insert(o1.Path())
	ps.vt.IncVersion(o1.Path())
	qid := ps.mkQid(o1.Perm(), o1.Path())
	nf := ps.makeFid(f.Pobj().Ctx(), f.Pobj().Path(), args.Name, o1, args.TleaseId(), qid)
	ps.ft.Add(args.Tfid(), nf)
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	nf.SetMode(args.Tmode())
	rets.Qid = qid
	return nil
}

func (ps *ProtSrv) ReadV(args *sp.TreadV, rets *sp.Rread) ([]byte, *sp.Rerror) {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return nil, sp.MkRerror(err)
	}
	v := ps.vt.GetVersion(f.Pobj().Obj().Path())
	db.DPrintf(db.PROTSRV, "%v: ReadV f %v args {%v} v %d", f.Pobj().Ctx().Uname(), f, args, v)
	if !sp.VEq(args.Tversion(), v) {
		return nil, sp.MkRerror(serr.MkErr(serr.TErrVersion, v))
	}

	data, err := f.Read(args.Toffset(), args.Tcount(), args.Tversion(), args.Tfence())
	if err != nil {
		return nil, sp.MkRerror(err)
	}
	return data, nil
}

func (ps *ProtSrv) WriteRead(args *sp.Twriteread, data []byte, rets *sp.Rread) ([]byte, *sp.Rerror) {
	f, err := ps.ft.Lookup(sp.Tfid(args.Fid))
	if err != nil {
		return nil, sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: WriteRead %v args {%v} path %d\n", f.Pobj().Ctx().Uname(), f.Pobj().Path(), args, f.Pobj().Obj().Path())
	retdata, err := f.WriteRead(data)
	if err != nil {
		return nil, sp.MkRerror(err)
	}
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	return retdata, nil
}

func (ps *ProtSrv) WriteV(args *sp.TwriteV, data []byte, rets *sp.Rwrite) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	v := ps.vt.GetVersion(f.Pobj().Obj().Path())
	db.DPrintf(db.PROTSRV, "%v: WriteV %v args {%v} path %d v %d", f.Pobj().Ctx().Uname(), f.Pobj().Path(), args, f.Pobj().Obj().Path(), v)
	if !sp.VEq(args.Tversion(), v) {
		return sp.MkRerror(serr.MkErr(serr.TErrVersion, v))
	}
	n, err := f.Write(args.Toffset(), data, args.Tversion(), args.Tfence())
	if err != nil {
		return sp.MkRerror(err)
	}
	rets.Count = uint32(n)
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	return nil
}

func (ps *ProtSrv) removeObj(ctx fs.CtxI, o fs.FsObj, path path.Path, f sp.Tfence) *sp.Rerror {
	name := path.Base()
	if name == "." {
		return sp.MkRerror(serr.MkErr(serr.TErrInval, name))
	}

	// lock path to make WatchV and Remove interact correctly
	dlk := ps.plt.Acquire(ctx, path.Dir())
	flk := ps.plt.Acquire(ctx, path)
	defer ps.plt.ReleaseLocks(ctx, dlk, flk)

	ps.stats.IncPathString(flk.Path())

	db.DPrintf(db.PROTSRV, "%v: removeObj %v in %v", ctx.Uname(), name, o)

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	ephemeral := o.Perm().IsEphemeral()
	err := o.Parent().Remove(ctx, name, f)
	if err != nil {
		return sp.MkRerror(err)
	}

	ps.vt.IncVersion(o.Path())
	ps.vt.IncVersion(o.Parent().Path())

	ps.wt.WakeupWatch(flk)
	ps.wt.WakeupWatch(dlk)

	if ephemeral && ps.et != nil {
		ps.et.Delete(path.String())
	}
	return nil
}

// Remove for backwards compatability; SigmaOS uses RemoveFile (see
// below) instead of Remove, but proxy will use it.
func (ps *ProtSrv) Remove(args *sp.Tremove, rets *sp.Rremove) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Remove %v", f.Pobj().Ctx().Uname(), f.Pobj().Path())
	return ps.removeObj(f.Pobj().Ctx(), f.Pobj().Obj(), f.Pobj().Path(), args.Tfence())
}

func (ps *ProtSrv) Stat(args *sp.Tstat, rets *sp.Rstat) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Stat %v", f.Pobj().Ctx().Uname(), f)
	ps.stats.IncPathString(f.Pobj().Path().String())
	o := f.Pobj().Obj()
	st, r := o.Stat(f.Pobj().Ctx())
	if r != nil {
		return sp.MkRerror(r)
	}
	rets.Stat = st
	return nil
}

//
// Rename: within the same directory (Wstat) and rename across directories
//

func (ps *ProtSrv) Wstat(args *sp.Twstat, rets *sp.Rwstat) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Wstat %v %v", f.Pobj().Ctx().Uname(), f, args)
	o := f.Pobj().Obj()
	if args.Stat.Name != "" {
		// update Name atomically with rename

		dst := f.Pobj().Path().Dir().Copy().AppendPath(path.Split(args.Stat.Name))

		dlk, slk := ps.plt.AcquireLocks(f.Pobj().Ctx(), f.Pobj().Path().Dir(), f.Pobj().Path().Base())
		defer ps.plt.ReleaseLocks(f.Pobj().Ctx(), dlk, slk)
		tlk := ps.plt.Acquire(f.Pobj().Ctx(), dst)
		defer ps.plt.Release(f.Pobj().Ctx(), tlk)
		ps.stats.IncPathString(f.Pobj().Path().String())
		err := o.Parent().Rename(f.Pobj().Ctx(), f.Pobj().Path().Base(), args.Stat.Name, args.Tfence())
		if err != nil {
			return sp.MkRerror(err)
		}
		ps.vt.IncVersion(f.Pobj().Obj().Path())
		ps.vt.IncVersion(f.Pobj().Obj().Parent().Path())
		ps.wt.WakeupWatch(tlk) // trigger create watch
		ps.wt.WakeupWatch(slk) // trigger remove watch
		ps.wt.WakeupWatch(dlk) // trigger dir watch
		if f.Pobj().Obj().Perm().IsEphemeral() && ps.et != nil {
			ps.et.Rename(f.Pobj().Path().String(), dst.String())
		}
		f.Pobj().SetPath(dst)
	}
	// XXX ignore other Wstat for now
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

func (ps *ProtSrv) Renameat(args *sp.Trenameat, rets *sp.Rrenameat) *sp.Rerror {
	oldf, err := ps.ft.Lookup(args.Toldfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	newf, err := ps.ft.Lookup(args.Tnewfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Renameat %v %v %v", oldf.Pobj().Ctx().Uname(), oldf, newf, args)
	oo := oldf.Pobj().Obj()
	no := newf.Pobj().Obj()
	switch d1 := oo.(type) {
	case fs.Dir:
		d2, ok := no.(fs.Dir)
		if !ok {
			return sp.MkRerror(serr.MkErr(serr.TErrNotDir, newf.Pobj().Path()))
		}
		if oo.Path() == no.Path() {
			return sp.MkRerror(serr.MkErr(serr.TErrInval, newf.Pobj().Path()))
		}

		var d1lk, d2lk, srclk, dstlk *lockmap.PathLock
		if srcfirst := lockOrder(oo, no); srcfirst {
			d1lk, srclk = ps.plt.AcquireLocks(oldf.Pobj().Ctx(), oldf.Pobj().Path(), args.OldName)
			d2lk, dstlk = ps.plt.AcquireLocks(newf.Pobj().Ctx(), newf.Pobj().Path(), args.NewName)
		} else {
			d2lk, dstlk = ps.plt.AcquireLocks(newf.Pobj().Ctx(), newf.Pobj().Path(), args.NewName)
			d1lk, srclk = ps.plt.AcquireLocks(oldf.Pobj().Ctx(), oldf.Pobj().Path(), args.OldName)
		}
		defer ps.plt.ReleaseLocks(oldf.Pobj().Ctx(), d1lk, srclk)
		defer ps.plt.ReleaseLocks(newf.Pobj().Ctx(), d2lk, dstlk)

		err := d1.Renameat(oldf.Pobj().Ctx(), args.OldName, d2, args.NewName, args.Tfence())
		if err != nil {
			return sp.MkRerror(err)
		}
		ps.vt.IncVersion(newf.Pobj().Obj().Path())
		ps.vt.IncVersion(oldf.Pobj().Obj().Path())

		ps.vt.IncVersion(oldf.Pobj().Obj().Parent().Path())
		ps.vt.IncVersion(newf.Pobj().Obj().Parent().Path())

		if oldf.Pobj().Obj().Perm().IsEphemeral() && ps.et != nil {
			ps.et.Rename(oldf.Pobj().Path().String(), newf.Pobj().Path().String())
		}

		ps.wt.WakeupWatch(dstlk) // trigger create watch
		ps.wt.WakeupWatch(srclk) // trigger remove watch
		ps.wt.WakeupWatch(d1lk)  // trigger one dir watch
		ps.wt.WakeupWatch(d2lk)  // trigger the other dir watch
	default:
		return sp.MkRerror(serr.MkErr(serr.TErrNotDir, oldf.Pobj().Path()))
	}
	return nil
}

func (ps *ProtSrv) lookupWalk(fid sp.Tfid, wnames path.Path, resolve bool) (*fid.Fid, path.Path, fs.FsObj, *serr.Err) {
	f, err := ps.ft.Lookup(fid)
	if err != nil {
		return nil, nil, nil, err
	}
	lo := f.Pobj().Obj()
	fname := append(f.Pobj().Path(), wnames...)
	if len(wnames) > 0 {
		lo, err = ps.lookupObjLast(f.Pobj().Ctx(), f, wnames, resolve)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return f, fname, lo, nil
}

func (ps *ProtSrv) lookupWalkOpen(fid sp.Tfid, wnames path.Path, resolve bool, mode sp.Tmode) (*fid.Fid, path.Path, fs.FsObj, fs.File, *serr.Err) {
	f, fname, lo, err := ps.lookupWalk(fid, wnames, resolve)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	no, err := lo.Open(f.Pobj().Ctx(), mode)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if no != nil {
		lo = no
	}
	i, err := fs.Obj2File(lo, fname)
	if err != nil {
		lo.Close(f.Pobj().Ctx(), mode)
		return nil, nil, nil, nil, err
	}
	return f, fname, lo, i, nil
}

func (ps *ProtSrv) RemoveFile(args *sp.Tremovefile, rets *sp.Rremove) *sp.Rerror {
	f, fname, lo, err := ps.lookupWalk(args.Tfid(), args.Wnames, args.Resolve)
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: RemoveFile %v %v %v", f.Pobj().Ctx().Uname(), f.Pobj().Path(), fname, args.Fid)
	return ps.removeObj(f.Pobj().Ctx(), lo, fname, args.Tfence())
}

func (ps *ProtSrv) GetFile(args *sp.Tgetfile, rets *sp.Rread) ([]byte, *sp.Rerror) {
	if args.Tcount() > sp.MAXGETSET {
		return nil, sp.MkRerror(serr.MkErr(serr.TErrInval, "too large"))
	}
	f, fname, lo, i, err := ps.lookupWalkOpen(args.Tfid(), args.Wnames, args.Resolve, args.Tmode())
	if err != nil {
		return nil, sp.MkRerror(err)
	}
	ps.stats.IncPathString(f.Pobj().Path().String())
	db.DPrintf(db.PROTSRV, "GetFile f %v args {%v} %v", f.Pobj().Ctx().Uname(), args, fname)
	data, err := i.Read(f.Pobj().Ctx(), args.Toffset(), args.Tcount(), sp.NoV, args.Tfence())
	if err != nil {
		return nil, sp.MkRerror(err)
	}
	if err := lo.Close(f.Pobj().Ctx(), args.Tmode()); err != nil {
		return nil, sp.MkRerror(err)
	}
	return data, nil
}

// Caller holds pathname lock for f
func (ps *ProtSrv) lookupPathOpen(f *fid.Fid, dir fs.Dir, name string, mode sp.Tmode, resolve bool) (fs.FsObj, *serr.Err) {
	_, lo, _, err := dir.LookupPath(f.Pobj().Ctx(), path.Path{name})
	if err != nil {
		return nil, err
	}
	if lo.Perm().IsSymlink() && resolve {
		return nil, serr.MkErr(serr.TErrNotDir, name)
	}
	no, err := lo.Open(f.Pobj().Ctx(), mode)
	if err != nil {
		return nil, err
	}
	if no != nil {
		lo = no
	}
	return lo, nil
}

// Create file or open file, and write data to it
func (ps *ProtSrv) PutFile(args *sp.Tputfile, data []byte, rets *sp.Rwrite) *sp.Rerror {
	if sp.Tsize(len(data)) > sp.MAXGETSET {
		return sp.MkRerror(serr.MkErr(serr.TErrInval, "too large"))
	}
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.MkRerror(err)
	}
	db.DPrintf(db.PROTSRV, "%v: PutFile o %v args {%v}", f.Pobj().Ctx().Uname(), f, args)
	fn := append(f.Pobj().Path(), args.Wnames...)
	dname := f.Pobj().Path().Dir()
	lo := f.Pobj().Obj()
	var dlk, flk *lockmap.PathLock
	if len(args.Wnames) > 0 {
		// walk to directory
		f, dname, lo, err = ps.lookupWalk(args.Tfid(), args.Wnames[0:len(args.Wnames)-1], false)
		if err != nil {
			return sp.MkRerror(err)
		}

		if !lo.Perm().IsDir() {
			return sp.MkRerror(serr.MkErr(serr.TErrNotDir, dname))
		}
		dlk = ps.plt.Acquire(f.Pobj().Ctx(), dname)
		defer ps.plt.Release(f.Pobj().Ctx(), dlk)

		db.DPrintf(db.PROTSRV, "%v: PutFile try to create %v", f.Pobj().Ctx().Uname(), fn)
		// try to create file, which will fail it exists
		dir := lo.(fs.Dir)
		lo, flk, err = ps.createObj(f.Pobj().Ctx(), dir, dlk, fn, args.Tperm(), args.Tmode(), args.TleaseId(), args.Tfence())
		if err != nil {
			if err.Code() != serr.TErrExists {
				return sp.MkRerror(err)
			}
			if err.Code() == serr.TErrExists && args.Tmode()&sp.OEXCL == sp.OEXCL {
				return sp.MkRerror(err)
			}
			db.DPrintf(db.PROTSRV, "%v: PutFile lookup %v", f.Pobj().Ctx().Uname(), fn.Base())
			// look up the file and get a lock on it. note: it cannot have
			// been removed since the failed create above, because PutFile
			// holds the directory lock.
			lo, err = ps.lookupPathOpen(f, dir, fn.Base(), args.Tmode(), args.Resolve)
			if err != nil {
				return sp.MkRerror(err)
			}
			// flk also ensures that two writes execute atomically
			flk = ps.plt.Acquire(f.Pobj().Ctx(), fn)
		}

	} else {
		db.DPrintf(db.PROTSRV, "%v: PutFile open %v (%v)", f.Pobj().Ctx().Uname(), fn, dname)
		dlk = ps.plt.Acquire(f.Pobj().Ctx(), dname)
		defer ps.plt.Release(f.Pobj().Ctx(), dlk)
		flk = ps.plt.Acquire(f.Pobj().Ctx(), fn)
		no, err := lo.Open(f.Pobj().Ctx(), args.Tmode())
		if err != nil {
			return sp.MkRerror(err)
		}
		if no != nil {
			lo = no
		}
	}
	defer ps.plt.Release(f.Pobj().Ctx(), flk)

	// make an fid for the file (in case we created it)
	qid := ps.mkQid(lo.Perm(), lo.Path())
	f = ps.makeFid(f.Pobj().Ctx(), dname, fn.Base(), lo, args.TleaseId(), qid)
	i, err := fs.Obj2File(lo, fn)
	if err != nil {
		return sp.MkRerror(err)
	}

	ps.stats.IncPathString(f.Pobj().Path().String())

	if args.Tmode()&sp.OAPPEND == sp.OAPPEND && args.Toffset() != sp.NoOffset {
		return sp.MkRerror(serr.MkErr(serr.TErrInval, "offset should be sp.NoOffset"))
	}
	if args.Toffset() == sp.NoOffset && args.Tmode()&sp.OAPPEND != sp.OAPPEND {
		return sp.MkRerror(serr.MkErr(serr.TErrInval, "mode shouldbe OAPPEND"))
	}

	n, err := i.Write(f.Pobj().Ctx(), args.Toffset(), data, sp.NoV, args.Tfence())
	if err != nil {
		return sp.MkRerror(err)
	}
	err = lo.Close(f.Pobj().Ctx(), args.Tmode())
	if err != nil {
		return sp.MkRerror(err)
	}
	rets.Count = uint32(n)
	return nil
}
