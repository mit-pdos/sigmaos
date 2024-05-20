package protsrv

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/fid"
	"sigmaos/fs"
	"sigmaos/lockmap"
	"sigmaos/namei"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/sesssrv"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type GetRootCtxF func(*sp.Tprincipal, map[string]*proc.ProcSecretProto, string, sessp.Tsession, sp.TclntId) (fs.Dir, fs.CtxI)

// Each session has its own protsrv, but they share ProtSrvState
type ProtSrv struct {
	*ProtSrvState
	ssrv       *sesssrv.SessSrv
	ft         *fidTable
	p          *sp.Tprincipal
	sid        sessp.Tsession
	getRootCtx GetRootCtxF
}

func NewProtServer(pss *ProtSrvState, p *sp.Tprincipal, sid sessp.Tsession, grf GetRootCtxF) sps.Protsrv {
	ps := &ProtSrv{
		ProtSrvState: pss,
		ft:           newFidTable(),
		p:            p,
		sid:          sid,
		getRootCtx:   grf,
	}
	db.DPrintf(db.PROTSRV, "NewProtSrv -> %v", ps)
	return ps
}

func (ps *ProtSrv) newQid(perm sp.Tperm, path sp.Tpath) *sp.Tqid {
	return sp.NewQidPerm(perm, ps.vt.GetVersion(path), path)
}

func (ps *ProtSrv) Version(args *sp.Tversion, rets *sp.Rversion) *sp.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (ps *ProtSrv) Auth(args *sp.Tauth, rets *sp.Rauth) *sp.Rerror {
	return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotSupported, "Auth"))
}

func (ps *ProtSrv) Attach(args *sp.Tattach, rets *sp.Rattach) (sp.TclntId, *sp.Rerror) {
	db.DPrintf(db.PROTSRV, "Attach %v cid %v sid %v", args, args.TclntId(), ps.sid)
	p := path.Split(args.Aname)
	// TODO: get secrest from attach ctx
	root, ctx := ps.getRootCtx(ps.p, nil, args.Aname, ps.sid, args.TclntId())
	tree := root.(fs.FsObj)
	qid := ps.newQid(tree.Perm(), tree.Path())
	if args.Aname != "" {
		dlk := ps.plt.Acquire(ctx, path.Path{}, lockmap.RLOCK)
		_, lo, lk, rest, err := namei.Walk(ps.plt, ctx, root, dlk, path.Path{}, p, nil, lockmap.RLOCK)
		defer ps.plt.Release(ctx, lk, lockmap.RLOCK)
		if len(rest) > 0 || err != nil {
			return sp.NoClntId, sp.NewRerrorSerr(err)
		}
		// insert before releasing
		ps.vt.Insert(lo.Path())
		tree = lo
		qid = ps.newQid(lo.Perm(), lo.Path())
	} else {
		// root is already in the version table; this updates
		// just the refcnt.
		ps.vt.Insert(root.Path())
	}
	ps.ft.Insert(args.Tfid(), fid.NewFidPath(fid.NewPobj(p, tree, ctx), 0, qid))
	rets.Qid = qid
	return args.TclntId(), nil
}

// Delete ephemeral files created by this client and delete this client
func (ps *ProtSrv) Detach(args *sp.Tdetach, rets *sp.Rdetach) *sp.Rerror {
	fids := ps.ft.ClientFids(args.TclntId())
	db.DPrintf(db.PROTSRV, "Detach clnt %v fes %v\n", args.TclntId(), fids)
	for _, fid := range fids {
		ps.clunk(fid)
	}
	// Several threads maybe waiting in a clntcond of this
	// clnt. DeleteClnt will unblock them so that they can bail out.
	ps.cct.DeleteClnt(args.TclntId())
	return nil
}

func (ps *ProtSrv) newQids(os []fs.FsObj) []*sp.Tqid {
	var qids []*sp.Tqid
	for _, o := range os {
		qids = append(qids, ps.newQid(o.Perm(), o.Path()))
	}
	return qids
}

func (ps *ProtSrv) lookupObjLast(ctx fs.CtxI, f *fid.Fid, names path.Path, resolve bool, ltype lockmap.Tlock) (fs.FsObj, *serr.Err) {
	_, lo, lk, _, err := ps.lookupObj(ctx, f.Pobj(), names, ltype)
	ps.plt.Release(ctx, lk, ltype)
	if err != nil {
		return nil, err
	}
	if lo.Perm().IsSymlink() && resolve {
		return nil, serr.NewErr(serr.TErrNotDir, names[len(names)-1])
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
		return sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: Walk o %v args {%v} (%v)", f.Pobj().Ctx().ClntId(), f, args, len(args.Wnames))

	s := time.Now()
	os, lo, lk, rest, err := ps.lookupObj(f.Pobj().Ctx(), f.Pobj(), args.Wnames, lockmap.RLOCK)
	db.DPrintf(db.WALK_LAT, "Walk %q %v lat %v\n", f.Pobj().Ctx().ClntId(), args.Wnames, time.Since(s))
	defer ps.plt.Release(f.Pobj().Ctx(), lk, lockmap.RLOCK)

	if lk != nil {
		ps.stats.IncPathString(lk.Path())
	}

	if err != nil && !err.IsMaybeSpecialElem() {
		return sp.NewRerrorSerr(err)
	}

	// let the client decide what to do with rest (when there is a rest)
	n := len(args.Wnames) - len(rest)
	p := append(f.Pobj().Path().Copy(), args.Wnames[:n]...)
	rets.Qids = ps.newQids(os)
	qid := ps.newQid(lo.Perm(), lo.Path())
	db.DPrintf(db.PROTSRV, "%v: Walk NewFidPath fid %v p %v lo %v qid %v os %v", args.NewFid, f.Pobj().Ctx().ClntId(), p, lo, qid, os)
	ps.ft.Insert(args.Tnewfid(), fid.NewFidPath(fid.NewPobj(p, lo, f.Pobj().Ctx()), 0, qid))

	ps.vt.Insert(qid.Tpath())

	return nil
}

func (ps *ProtSrv) clunk(fid sp.Tfid) *sp.Rerror {
	f, err := ps.ft.LookupDel(fid)
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Clunk %v f %v path %q", f.Pobj().Ctx().ClntId(), fid, f, f.Pobj().Path())
	if f.IsOpen() { // has the fid been opened?
		f.Pobj().Obj().Close(f.Pobj().Ctx(), f.Mode())
		f.Close()
	}
	if _, err := ps.vt.Delete(f.Pobj().Obj().Path()); err != nil {
		db.DFatalf("%v: clunk %v vt del failed %v err %v\n", f.Pobj().Ctx().ClntId(), fid, f.Pobj(), err)
	}
	return nil
}

func (ps *ProtSrv) Clunk(args *sp.Tclunk, rets *sp.Rclunk) *sp.Rerror {
	return ps.clunk(args.Tfid())
}

func (ps *ProtSrv) Open(args *sp.Topen, rets *sp.Ropen) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Open f %v %v", f.Pobj().Ctx().ClntId(), f, args)

	ps.stats.IncPathString(f.Pobj().Path().String())

	o := f.Pobj().Obj()
	no, r := o.Open(f.Pobj().Ctx(), args.Tmode())
	if r != nil {
		return sp.NewRerrorSerr(r)
	}
	f.SetMode(args.Tmode())
	if no != nil {
		f.Pobj().SetObj(no)
		ps.vt.Insert(no.Path())
		ps.vt.IncVersion(no.Path())
		rets.Qid = ps.newQid(no.Perm(), no.Path())
	} else {
		rets.Qid = ps.newQid(o.Perm(), o.Path())
	}
	return nil
}

func (ps *ProtSrv) Watch(args *sp.Twatch, rets *sp.Ropen) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	p := f.Pobj().Path()
	ino := f.Pobj().Obj().Path()

	db.DPrintf(db.PROTSRV, "%v: Watch %v v %v %v", f.Pobj().Ctx().ClntId(), f.Pobj().Path(), f.Qid(), args)

	// get path lock on for p, so that remove cannot remove file
	// before watch is set.
	pl := ps.plt.Acquire(f.Pobj().Ctx(), p, lockmap.WLOCK)
	defer ps.plt.Release(f.Pobj().Ctx(), pl, lockmap.WLOCK)

	v := ps.vt.GetVersion(ino)
	if !sp.VEq(f.Qid().Tversion(), v) {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrVersion, v))
	}
	err = ps.wt.WaitWatch(pl, f.Pobj().Ctx().ClntId())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	return nil
}

func (ps *ProtSrv) newFid(ctx fs.CtxI, dir path.Path, name string, o fs.FsObj, lid sp.TleaseId, qid *sp.Tqid) *fid.Fid {
	pn := dir.Copy().Append(name)
	po := fid.NewPobj(pn, o, ctx)
	nf := fid.NewFidPath(po, 0, qid)
	if o.Perm().IsEphemeral() && ps.et != nil {
		ps.et.Insert(pn.String(), lid)
	}
	return nf
}

// Create name in dir and returns lock for it.
func (ps *ProtSrv) createObj(ctx fs.CtxI, d fs.Dir, dlk *lockmap.PathLock, fn path.Path, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (fs.FsObj, *lockmap.PathLock, *serr.Err) {
	name := fn.Base()
	if name == "." {
		return nil, nil, serr.NewErr(serr.TErrInval, name)
	}
	flk := ps.plt.Acquire(ctx, fn, lockmap.WLOCK)
	o1, err := d.Create(ctx, name, perm, mode, lid, f)
	db.DPrintf(db.PROTSRV, "%v: Create %q %v %v ephemeral %v %v lid %v", ctx.ClntId(), name, o1, err, perm.IsEphemeral(), ps.sid, lid)
	if err == nil {
		ps.wt.WakeupWatch(dlk)
		return o1, flk, nil
	} else {
		ps.plt.Release(ctx, flk, lockmap.WLOCK)
		return nil, nil, err
	}
}

func (ps *ProtSrv) Create(args *sp.Tcreate, rets *sp.Rcreate) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Create f %v", f.Pobj().Ctx().ClntId(), f)
	o := f.Pobj().Obj()
	fn := f.Pobj().Path().Append(args.Name)
	if !o.Perm().IsDir() {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, f.Pobj().Path()))
	}
	d := o.(fs.Dir)
	dlk := ps.plt.Acquire(f.Pobj().Ctx(), f.Pobj().Path(), lockmap.WLOCK)
	defer ps.plt.Release(f.Pobj().Ctx(), dlk, lockmap.WLOCK)

	o1, flk, err := ps.createObj(f.Pobj().Ctx(), d, dlk, fn, args.Tperm(), args.Tmode(), args.TleaseId(), args.Tfence())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	defer ps.plt.Release(f.Pobj().Ctx(), flk, lockmap.WLOCK)
	ps.stats.IncPathString(f.Pobj().Path().String())
	ps.vt.Insert(o1.Path())
	ps.vt.IncVersion(o1.Path())
	qid := ps.newQid(o1.Perm(), o1.Path())
	nf := ps.newFid(f.Pobj().Ctx(), f.Pobj().Path(), args.Name, o1, args.TleaseId(), qid)
	ps.ft.Insert(args.Tfid(), nf)
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	nf.SetMode(args.Tmode())
	rets.Qid = qid
	return nil
}

func (ps *ProtSrv) ReadF(args *sp.TreadF, rets *sp.Rread) ([]byte, *sp.Rerror) {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: ReadF f %v args {%v}\n", f.Pobj().Ctx().ClntId(), f, args)

	flk := ps.plt.Acquire(f.Pobj().Ctx(), f.Pobj().Path(), lockmap.RLOCK)
	defer ps.plt.Release(f.Pobj().Ctx(), flk, lockmap.RLOCK)

	data, err := f.Read(args.Toffset(), args.Tcount(), args.Tfence())
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(len(data))
	return data, nil
}

func (ps *ProtSrv) WriteRead(args *sp.Twriteread, iov sessp.IoVec, rets *sp.Rread) (sessp.IoVec, *sp.Rerror) {
	f, err := ps.ft.Lookup(sp.Tfid(args.Fid))
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: WriteRead %v args {%v} path %d\n", f.Pobj().Ctx().ClntId(), f.Pobj().Path(), args, f.Pobj().Obj().Path())
	retiov, err := f.WriteRead(iov)
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	return retiov, nil
}

func (ps *ProtSrv) WriteF(args *sp.TwriteF, data []byte, rets *sp.Rwrite) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: WriteV %v args {%v} path %d\n", f.Pobj().Ctx().ClntId(), f.Pobj().Path(), args, f.Pobj().Obj().Path())

	n, err := f.Write(args.Toffset(), data, args.Tfence())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(n)
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	return nil
}

func (ps *ProtSrv) removeObj(ctx fs.CtxI, o fs.FsObj, path path.Path, f sp.Tfence) *sp.Rerror {
	name := path.Base()
	if name == "." {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, name))
	}

	// lock path to make WatchV and Remove interact correctly
	dlk := ps.plt.Acquire(ctx, path.Dir(), lockmap.WLOCK)
	flk := ps.plt.Acquire(ctx, path, lockmap.WLOCK)
	defer ps.plt.ReleaseLocks(ctx, dlk, flk, lockmap.WLOCK)

	ps.stats.IncPathString(flk.Path())

	db.DPrintf(db.PROTSRV, "%v: removeObj %v in %v", ctx.ClntId(), name, o)

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	ephemeral := o.Perm().IsEphemeral()
	err := o.Parent().Remove(ctx, name, f)
	if err != nil {
		return sp.NewRerrorSerr(err)
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
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Remove %v", f.Pobj().Ctx().ClntId(), f.Pobj().Path())
	return ps.removeObj(f.Pobj().Ctx(), f.Pobj().Obj(), f.Pobj().Path(), args.Tfence())
}

func (ps *ProtSrv) Stat(args *sp.Trstat, rets *sp.Rrstat) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Stat %v", f.Pobj().Ctx().ClntId(), f)
	ps.stats.IncPathString(f.Pobj().Path().String())
	o := f.Pobj().Obj()
	st, r := o.Stat(f.Pobj().Ctx())
	if r != nil {
		return sp.NewRerrorSerr(r)
	}
	rets.Stat = st.StatProto()
	return nil
}

//
// Rename: within the same directory (Wstat) and rename across directories
//

func (ps *ProtSrv) Wstat(args *sp.Twstat, rets *sp.Rwstat) *sp.Rerror {
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Wstat %v %v", f.Pobj().Ctx().ClntId(), f, args)
	o := f.Pobj().Obj()
	if args.Stat.Name != "" {
		// update Name atomically with rename

		dst := f.Pobj().Path().Dir().Copy().AppendPath(path.Split(args.Stat.Name))

		dlk, slk := ps.plt.AcquireLocks(f.Pobj().Ctx(), f.Pobj().Path().Dir(), f.Pobj().Path().Base(), lockmap.WLOCK)
		defer ps.plt.ReleaseLocks(f.Pobj().Ctx(), dlk, slk, lockmap.WLOCK)
		tlk := ps.plt.Acquire(f.Pobj().Ctx(), dst, lockmap.WLOCK)
		defer ps.plt.Release(f.Pobj().Ctx(), tlk, lockmap.WLOCK)
		ps.stats.IncPathString(f.Pobj().Path().String())
		err := o.Parent().Rename(f.Pobj().Ctx(), f.Pobj().Path().Base(), args.Stat.Name, args.Tfence())
		if err != nil {
			return sp.NewRerrorSerr(err)
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
		return sp.NewRerrorSerr(err)
	}
	newf, err := ps.ft.Lookup(args.Tnewfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Renameat %v %v %v", oldf.Pobj().Ctx().ClntId(), oldf, newf, args)
	oo := oldf.Pobj().Obj()
	no := newf.Pobj().Obj()
	switch d1 := oo.(type) {
	case fs.Dir:
		d2, ok := no.(fs.Dir)
		if !ok {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, newf.Pobj().Path()))
		}
		if oo.Path() == no.Path() {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, newf.Pobj().Path()))
		}

		var d1lk, d2lk, srclk, dstlk *lockmap.PathLock
		if srcfirst := lockOrder(oo, no); srcfirst {
			d1lk, srclk = ps.plt.AcquireLocks(oldf.Pobj().Ctx(), oldf.Pobj().Path(), args.OldName, lockmap.WLOCK)
			d2lk, dstlk = ps.plt.AcquireLocks(newf.Pobj().Ctx(), newf.Pobj().Path(), args.NewName, lockmap.WLOCK)
		} else {
			d2lk, dstlk = ps.plt.AcquireLocks(newf.Pobj().Ctx(), newf.Pobj().Path(), args.NewName, lockmap.WLOCK)
			d1lk, srclk = ps.plt.AcquireLocks(oldf.Pobj().Ctx(), oldf.Pobj().Path(), args.OldName, lockmap.WLOCK)
		}
		defer ps.plt.ReleaseLocks(oldf.Pobj().Ctx(), d1lk, srclk, lockmap.WLOCK)
		defer ps.plt.ReleaseLocks(newf.Pobj().Ctx(), d2lk, dstlk, lockmap.WLOCK)

		err := d1.Renameat(oldf.Pobj().Ctx(), args.OldName, d2, args.NewName, args.Tfence())
		if err != nil {
			return sp.NewRerrorSerr(err)
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
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, oldf.Pobj().Path()))
	}
	return nil
}

func (ps *ProtSrv) lookupWalk(fid sp.Tfid, wnames path.Path, resolve bool, ltype lockmap.Tlock) (*fid.Fid, path.Path, fs.FsObj, *serr.Err) {
	f, err := ps.ft.Lookup(fid)
	if err != nil {
		return nil, nil, nil, err
	}
	lo := f.Pobj().Obj()
	fname := append(f.Pobj().Path(), wnames...)
	if len(wnames) > 0 {
		lo, err = ps.lookupObjLast(f.Pobj().Ctx(), f, wnames, resolve, ltype)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return f, fname, lo, nil
}

func (ps *ProtSrv) lookupWalkOpen(fid sp.Tfid, wnames path.Path, resolve bool, mode sp.Tmode, ltype lockmap.Tlock) (*fid.Fid, path.Path, fs.FsObj, fs.File, *serr.Err) {
	f, fname, lo, err := ps.lookupWalk(fid, wnames, resolve, ltype)
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
	f, fname, lo, err := ps.lookupWalk(args.Tfid(), args.Wnames, args.Resolve, lockmap.WLOCK)
	if err != nil {
		db.DPrintf(db.PROTSRV, "RemoveFile %v err %v", args, err)
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: RemoveFile %v %v %v", f.Pobj().Ctx().ClntId(), f.Pobj().Path(), fname, args.Fid)
	return ps.removeObj(f.Pobj().Ctx(), lo, fname, args.Tfence())
}

func (ps *ProtSrv) GetFile(args *sp.Tgetfile, rets *sp.Rread) ([]byte, *sp.Rerror) {
	if args.Tcount() > sp.MAXGETSET {
		return nil, sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "too large"))
	}
	f, fname, lo, i, err := ps.lookupWalkOpen(args.Tfid(), args.Wnames, args.Resolve, args.Tmode(), lockmap.RLOCK)
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	ps.stats.IncPathString(f.Pobj().Path().String())
	db.DPrintf(db.PROTSRV, "GetFile f %v args {%v} %v", f.Pobj().Ctx().ClntId(), args, fname)
	data, err := i.Read(f.Pobj().Ctx(), args.Toffset(), args.Tcount(), args.Tfence())
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	if err := lo.Close(f.Pobj().Ctx(), args.Tmode()); err != nil {
		return nil, sp.NewRerrorSerr(err)
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
		return nil, serr.NewErr(serr.TErrNotDir, name)
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
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "too large"))
	}
	f, err := ps.ft.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: PutFile o %v args {%v}", f.Pobj().Ctx().ClntId(), f, args)
	fn := append(f.Pobj().Path(), args.Wnames...)
	dname := f.Pobj().Path().Dir()
	lo := f.Pobj().Obj()
	var dlk, flk *lockmap.PathLock
	if len(args.Wnames) > 0 {
		// walk to directory
		f, dname, lo, err = ps.lookupWalk(args.Tfid(), args.Wnames[0:len(args.Wnames)-1], false, lockmap.WLOCK)
		if err != nil {
			return sp.NewRerrorSerr(err)
		}

		if !lo.Perm().IsDir() {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, dname))
		}
		dlk = ps.plt.Acquire(f.Pobj().Ctx(), dname, lockmap.WLOCK)
		defer ps.plt.Release(f.Pobj().Ctx(), dlk, lockmap.WLOCK)

		db.DPrintf(db.PROTSRV, "%v: PutFile try to create %v", f.Pobj().Ctx().ClntId(), fn)
		// try to create file, which will fail it exists
		dir := lo.(fs.Dir)
		lo, flk, err = ps.createObj(f.Pobj().Ctx(), dir, dlk, fn, args.Tperm(), args.Tmode(), args.TleaseId(), args.Tfence())
		if err != nil {
			if err.Code() != serr.TErrExists {
				return sp.NewRerrorSerr(err)
			}
			if err.Code() == serr.TErrExists && args.Tmode()&sp.OEXCL == sp.OEXCL {
				return sp.NewRerrorSerr(err)
			}
			db.DPrintf(db.PROTSRV, "%v: PutFile lookup %v", f.Pobj().Ctx().ClntId(), fn.Base())
			// look up the file and get a lock on it. note: it cannot have
			// been removed since the failed create above, because PutFile
			// holds the directory lock.
			lo, err = ps.lookupPathOpen(f, dir, fn.Base(), args.Tmode(), args.Resolve)
			if err != nil {
				return sp.NewRerrorSerr(err)
			}
			// flk also ensures that two writes execute atomically
			flk = ps.plt.Acquire(f.Pobj().Ctx(), fn, lockmap.WLOCK)
		}

	} else {
		db.DPrintf(db.PROTSRV, "%v: PutFile open %v (%v)", f.Pobj().Ctx().ClntId(), fn, dname)
		dlk = ps.plt.Acquire(f.Pobj().Ctx(), dname, lockmap.WLOCK)
		defer ps.plt.Release(f.Pobj().Ctx(), dlk, lockmap.WLOCK)
		flk = ps.plt.Acquire(f.Pobj().Ctx(), fn, lockmap.WLOCK)
		no, err := lo.Open(f.Pobj().Ctx(), args.Tmode())
		if err != nil {
			return sp.NewRerrorSerr(err)
		}
		if no != nil {
			lo = no
		}
	}
	defer ps.plt.Release(f.Pobj().Ctx(), flk, lockmap.WLOCK)

	// make an fid for the file (in case we created it)
	qid := ps.newQid(lo.Perm(), lo.Path())
	f = ps.newFid(f.Pobj().Ctx(), dname, fn.Base(), lo, args.TleaseId(), qid)
	i, err := fs.Obj2File(lo, fn)
	if err != nil {
		return sp.NewRerrorSerr(err)
	}

	ps.stats.IncPathString(f.Pobj().Path().String())

	if args.Tmode()&sp.OAPPEND == sp.OAPPEND && args.Toffset() != sp.NoOffset {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "offset should be sp.NoOffset"))
	}
	if args.Toffset() == sp.NoOffset && args.Tmode()&sp.OAPPEND != sp.OAPPEND {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "mode shouldbe OAPPEND"))
	}

	n, err := i.Write(f.Pobj().Ctx(), args.Toffset(), data, args.Tfence())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	err = lo.Close(f.Pobj().Ctx(), args.Tmode())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(n)
	return nil
}
