package protsrv

import (
	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/sesssrv"
	"ulambda/stats"
	"ulambda/version"
	"ulambda/watch"
)

//
// There is one protsrv per session, but they share the watch table,
// and stats across sessions.  Each session has its own fid table,
// ephemeral table, and lease table.
//

type ProtSrv struct {
	ssrv  *sesssrv.SessSrv
	wt    *watch.WatchTable     // shared across sessions
	vt    *version.VersionTable // shared across sessions
	ft    *fidTable
	et    *ephemeralTable
	stats *stats.Stats
	sid   np.Tsession
}

func MakeProtServer(s np.SessServer, sid np.Tsession) np.Protsrv {
	ps := &ProtSrv{}
	srv := s.(*sesssrv.SessSrv)
	ps.ssrv = srv

	ps.ft = makeFidTable()
	ps.et = makeEphemeralTable()
	ps.wt = srv.GetWatchTable()
	ps.vt = srv.GetVersionTable()
	ps.stats = srv.GetStats()
	ps.sid = sid
	db.DPrintf("PROTSRV", "MakeProtSrv -> %v", ps)
	return ps
}

func (ps *ProtSrv) mkQid(perm np.Tperm, path np.Tpath) np.Tqid {
	return np.MakeQidPerm(perm, ps.vt.GetVersion(path), path)
}

func (ps *ProtSrv) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (ps *ProtSrv) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.MkErr(np.TErrNotSupported, "Auth").Rerror()
}

func (ps *ProtSrv) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	db.DPrintf("PROTSRV", "Attach %v\n", args)
	path := np.Split(args.Aname)
	root, ctx := ps.ssrv.AttachTree(args.Uname, args.Aname, ps.sid)
	tree := root.(fs.FsObj)
	qid := ps.mkQid(tree.Perm(), tree.Path())
	if args.Aname != "" {
		_, lo, fws, rest, err := ps.namei(ctx, root, np.Path{}, path, nil)
		if len(rest) > 0 || err != nil {
			return err.Rerror()
		}
		ps.wt.Release(fws)
		tree = lo
		qid = ps.mkQid(lo.Perm(), lo.Path())
	}
	ps.ft.Add(args.Fid, fid.MakeFidPath(fid.MkPobj(path, tree, ctx), 0, qid))
	ps.vt.Insert(tree.Path())

	rets.Qid = qid
	return nil
}

// Delete ephemeral files created on this session.
func (ps *ProtSrv) Detach(rets *np.Rdetach) *np.Rerror {
	db.DPrintf("PROTSRV", "Detach %v eph %v\n", ps.sid, ps.et.Get())

	// Several threads maybe waiting in a sesscond. DeleteSess
	// will unblock them so that they can bail out.
	ps.ssrv.GetSessCondTable().DeleteSess(ps.sid)

	ps.ft.ClunkOpen()
	ephemeral := ps.et.Get()
	for _, po := range ephemeral {
		db.DPrintf("PROTSRV", "Detach %v\n", po.Path())
		ps.removeObj(po.Ctx(), po.Obj(), po.Path())
	}
	return nil
}

func (ps *ProtSrv) makeQids(os []fs.FsObj) []np.Tqid {
	var qids []np.Tqid
	for _, o := range os {
		qids = append(qids, ps.mkQid(o.Perm(), o.Path()))
	}
	return qids
}

func (ps *ProtSrv) lookupObjLast(ctx fs.CtxI, f *fid.Fid, names np.Path, resolve bool) (fs.FsObj, *np.Err) {
	_, lo, fws, _, err := ps.lookupObj(ctx, f.Pobj(), names)
	if err != nil {
		return nil, err
	}
	defer ps.wt.Release(fws)
	if lo.Perm().IsSymlink() && resolve {
		return nil, np.MkErr(np.TErrNotDir, names[len(names)-1])
	}
	return lo, nil
}

// Requests that combine walk, open, and do operation in a single RPC,
// which also avoids clunking. They may fail because args.Wnames may
// contains a special path element; in that, case the client must walk
// args.Wnames.
func (ps *ProtSrv) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}

	db.DPrintf("PROTSRV", "%v: Walk o %v args %v (%v)\n", f.Pobj().Ctx().Uname(), f, args, len(args.Wnames))

	os, lo, fws, rest, err := ps.lookupObj(f.Pobj().Ctx(), f.Pobj(), args.Wnames)
	if err != nil && !np.IsMaybeSpecialElem(err) {
		return err.Rerror()
	}

	// let the client decide what to do with rest
	n := len(args.Wnames) - len(rest)
	p := append(f.Pobj().Path(), args.Wnames[:n]...)
	rets.Qids = ps.makeQids(os)
	qid := ps.mkQid(f.Pobj().Obj().Perm(), f.Pobj().Obj().Path())
	if len(os) == 0 { // cloning f into args.NewFid in ft
		lo = f.Pobj().Obj()
	} else {
		qid = ps.mkQid(lo.Perm(), lo.Path())
	}
	db.DPrintf("PROTSRV", "%v: Walk MakeFidPath fid %v p %v lo %v qid %v os %v", args.NewFid, f.Pobj().Ctx().Uname(), p, lo, qid, os)
	ps.ft.Add(args.NewFid, fid.MakeFidPath(fid.MkPobj(p, lo, f.Pobj().Ctx()), 0, qid))
	ps.vt.Insert(qid.Path)
	if fws != nil {
		ps.wt.Release(fws)
	}
	return nil
}

func (ps *ProtSrv) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: Clunk %v %v\n", f.Pobj().Ctx().Uname(), args.Fid, f)
	if f.IsOpen() { // has the fid been opened?
		f.Pobj().Obj().Close(f.Pobj().Ctx(), f.Mode())
		f.Close()
	}
	ps.ft.Del(args.Fid)
	ps.vt.Delete(f.Pobj().Obj().Path())
	return nil
}

func (ps *ProtSrv) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: Open f %v %v\n", f.Pobj().Ctx().Uname(), f, args)

	o := f.Pobj().Obj()
	no, r := o.Open(f.Pobj().Ctx(), args.Mode)
	if r != nil {
		return r.Rerror()
	}
	f.SetMode(args.Mode)
	if no != nil {
		f.Pobj().SetObj(no)
		rets.Qid = ps.mkQid(no.Perm(), no.Path())
	} else {
		rets.Qid = ps.mkQid(o.Perm(), o.Path())
	}
	return nil
}

func (ps *ProtSrv) Watch(args np.Twatch, rets *np.Ropen) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	p := f.Pobj().Path()
	ino := f.Pobj().Obj().Path()

	db.DPrintf("PROTSRV", "%v: Watch %v v %v %v\n", f.Pobj().Ctx().Uname(), f.Pobj().Path(), f.Qid(), args)

	// get lock on watch entry for p, so that remove cannot remove
	// file before watch is set.
	ws := ps.wt.WatchLookupL(p)
	defer ps.wt.Release(ws)

	v := ps.vt.GetVersion(ino)
	if !np.VEq(f.Qid().Version, v) {
		return np.MkErr(np.TErrVersion, v).Rerror()
	}
	// time.Sleep(1000 * time.Nanosecond)

	err = ws.Watch(ps.sid)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (ps *ProtSrv) makeFid(ctx fs.CtxI, dir np.Path, name string, o fs.FsObj, eph bool, qid np.Tqid) *fid.Fid {
	p := dir.Copy()
	po := fid.MkPobj(append(p, name), o, ctx)
	nf := fid.MakeFidPath(po, 0, qid)
	if eph {
		ps.et.Add(o, po)
	}
	return nf
}

// Create name in dir. If OWATCH is set and name already exits, wait
// until another thread deletes it, and retry.
func (ps *ProtSrv) createObj(ctx fs.CtxI, d fs.Dir, dws, fws *watch.Watch, name string, perm np.Tperm, mode np.Tmode) (fs.FsObj, *np.Err) {
	if name == "." {
		return nil, np.MkErr(np.TErrInval, name)
	}
	for {
		o1, err := d.Create(ctx, name, perm, mode)
		db.DPrintf("PROTSRV", "%v: Create %v %v %v ephemeral %v %v\n", ctx.Uname(), name, o1, err, perm.IsEphemeral(), ps.sid)
		if err == nil {
			fws.WakeupWatchL()
			dws.WakeupWatchL()
			return o1, nil
		} else {
			if mode&np.OWATCH == np.OWATCH && err.Code() == np.TErrExists {
				fws.Unlock()
				err := dws.Watch(ps.sid)
				fws.Lock() // not necessary if fail, but nicer with defer
				if err != nil {
					return nil, err
				}
				// try again; we will hold lock on watchers
			} else {
				return nil, err
			}
		}
	}
}

func (ps *ProtSrv) AcquireWatches(dir np.Path, file string) (*watch.Watch, *watch.Watch) {
	dws := ps.wt.WatchLookupL(dir)
	fws := ps.wt.WatchLookupL(append(dir, file))
	return dws, fws
}

func (ps *ProtSrv) ReleaseWatches(dws, fws *watch.Watch) {
	ps.wt.Release(dws)
	ps.wt.Release(fws)
}

func (ps *ProtSrv) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: Create f %v\n", f.Pobj().Ctx().Uname(), f)
	o := f.Pobj().Obj()
	names := np.Path{args.Name}
	if !o.Perm().IsDir() {
		return np.MkErr(np.TErrNotDir, f.Pobj().Path()).Rerror()
	}
	d := o.(fs.Dir)
	dws, fws := ps.AcquireWatches(f.Pobj().Path(), names[0])
	defer ps.ReleaseWatches(dws, fws)

	o1, err := ps.createObj(f.Pobj().Ctx(), d, dws, fws, names[0], args.Perm, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	ps.vt.Insert(o1.Path())
	ps.vt.IncVersion(o1.Path())
	qid := ps.mkQid(o1.Perm(), o1.Path())
	nf := ps.makeFid(f.Pobj().Ctx(), f.Pobj().Path(), names[0], o1, args.Perm.IsEphemeral(), qid)
	ps.ft.Add(args.Fid, nf)
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	nf.SetMode(args.Mode)
	rets.Qid = qid
	return nil
}

func (ps *ProtSrv) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (ps *ProtSrv) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: Read f %v args %v\n", f.Pobj().Ctx().Uname(), f, args)
	err = f.Read(args.Offset, args.Count, np.NoV, rets)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (ps *ProtSrv) ReadV(args np.TreadV, rets *np.Rread) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	v := ps.vt.GetVersion(f.Pobj().Obj().Path())
	db.DPrintf("PROTSRV0", "%v: ReadV f %v args %v v %d\n", f.Pobj().Ctx().Uname(), f, args, v)
	if !np.VEq(args.Version, v) {
		return np.MkErr(np.TErrVersion, v).Rerror()
	}

	err = f.Read(args.Offset, args.Count, args.Version, rets)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (ps *ProtSrv) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	rets.Count, err = f.Write(args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	return nil
}

func (ps *ProtSrv) WriteV(args np.TwriteV, rets *np.Rwrite) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	v := ps.vt.GetVersion(f.Pobj().Obj().Path())
	db.DPrintf("PROTSRV0", "%v: WriteV %v args %v path %d v %d\n", f.Pobj().Ctx().Uname(), f.Pobj().Path(), args, f.Pobj().Obj().Path(), v)
	if !np.VEq(args.Version, v) {
		return np.MkErr(np.TErrVersion, v).Rerror()
	}
	rets.Count, err = f.Write(args.Offset, args.Data, args.Version)
	if err != nil {
		return err.Rerror()
	}
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	return nil
}

func (ps *ProtSrv) removeObj(ctx fs.CtxI, o fs.FsObj, path np.Path) *np.Rerror {
	name := path.Base()
	if name == "." {
		return np.MkErr(np.TErrInval, name).Rerror()
	}

	// lock watch entry to make WatchV and Remove interact
	// correctly
	dws := ps.wt.WatchLookupL(path.Dir())
	fws := ps.wt.WatchLookupL(path)
	defer ps.wt.Release(dws)
	defer ps.wt.Release(fws)

	ps.stats.IncPath(path)

	db.DPrintf("PROTSRV", "%v: removeObj %v in %v", ctx.Uname(), name, o)

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	ephemeral := o.Perm().IsEphemeral()
	err := o.Parent().Remove(ctx, name)
	if err != nil {
		return err.Rerror()
	}

	ps.vt.IncVersion(o.Path())
	ps.vt.IncVersion(o.Parent().Path())

	fws.WakeupWatchL()
	dws.WakeupWatchL()

	if ephemeral {
		ps.et.Del(o)
	}
	return nil
}

// Remove for backwards compatability; SigmaOS uses RemoveFile (see
// below) instead of Remove, but proxy will use it.
func (ps *ProtSrv) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: Remove %v\n", f.Pobj().Ctx().Uname(), f.Pobj().Path())
	return ps.removeObj(f.Pobj().Ctx(), f.Pobj().Obj(), f.Pobj().Path())
}

func (ps *ProtSrv) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: Stat %v\n", f.Pobj().Ctx().Uname(), f)
	o := f.Pobj().Obj()
	st, r := o.Stat(f.Pobj().Ctx())
	if r != nil {
		return r.Rerror()
	}
	rets.Stat = *st
	return nil
}

//
// Rename: within the same directory (Wstat) and rename across directories
//

func (ps *ProtSrv) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: Wstat %v %v\n", f.Pobj().Ctx().Uname(), f, args)
	o := f.Pobj().Obj()
	if args.Stat.Name != "" {
		// update Name atomically with rename

		dst := f.Pobj().Path().Dir().Copy().AppendPath(np.Split(args.Stat.Name))

		dws, sws := ps.AcquireWatches(f.Pobj().Path().Dir(), f.Pobj().Path().Base())
		defer ps.ReleaseWatches(dws, sws)
		tws := ps.wt.WatchLookupL(dst)
		defer ps.wt.Release(tws)

		err := o.Parent().Rename(f.Pobj().Ctx(), f.Pobj().Path().Base(), args.Stat.Name)
		if err != nil {
			return err.Rerror()
		}
		ps.vt.IncVersion(f.Pobj().Obj().Path())
		tws.WakeupWatchL() // trigger create watch
		sws.WakeupWatchL() // trigger remove watch
		dws.WakeupWatchL() // trigger dir watch
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

func (ps *ProtSrv) Renameat(args np.Trenameat, rets *np.Rrenameat) *np.Rerror {
	oldf, err := ps.ft.Lookup(args.OldFid)
	if err != nil {
		return err.Rerror()
	}
	newf, err := ps.ft.Lookup(args.NewFid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: renameat %v %v %v\n", oldf.Pobj().Ctx().Uname(), oldf, newf, args)
	oo := oldf.Pobj().Obj()
	no := newf.Pobj().Obj()
	switch d1 := oo.(type) {
	case fs.Dir:
		d2, ok := no.(fs.Dir)
		if !ok {
			return np.MkErr(np.TErrNotDir, newf.Pobj().Path()).Rerror()
		}
		if oo.Path() == no.Path() {
			return np.MkErr(np.TErrInval, newf.Pobj().Path()).Rerror()
		}

		var d1ws, d2ws, srcws, dstws *watch.Watch
		if srcfirst := lockOrder(oo, no); srcfirst {
			d1ws, srcws = ps.AcquireWatches(oldf.Pobj().Path(), args.OldName)
			d2ws, dstws = ps.AcquireWatches(newf.Pobj().Path(), args.NewName)
		} else {
			d2ws, dstws = ps.AcquireWatches(newf.Pobj().Path(), args.NewName)
			d1ws, srcws = ps.AcquireWatches(oldf.Pobj().Path(), args.OldName)
		}
		defer ps.ReleaseWatches(d1ws, srcws)
		defer ps.ReleaseWatches(d2ws, dstws)

		err := d1.Renameat(oldf.Pobj().Ctx(), args.OldName, d2, args.NewName)
		if err != nil {
			return err.Rerror()
		}
		ps.vt.IncVersion(newf.Pobj().Obj().Path())
		ps.vt.IncVersion(oldf.Pobj().Obj().Path())

		dstws.WakeupWatchL() // trigger create watch
		srcws.WakeupWatchL() // trigger remove watch
		d1ws.WakeupWatchL()  // trigger one dir watch
		d2ws.WakeupWatchL()  // trigger the other dir watch
	default:
		return np.MkErr(np.TErrNotDir, oldf.Pobj().Path()).Rerror()
	}
	return nil
}

func (ps *ProtSrv) lookupWalk(fid np.Tfid, wnames np.Path, resolve bool) (*fid.Fid, np.Path, fs.FsObj, *np.Err) {
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

func (ps *ProtSrv) lookupWalkOpen(fid np.Tfid, wnames np.Path, resolve bool, mode np.Tmode) (*fid.Fid, np.Path, fs.FsObj, fs.File, *np.Err) {
	f, fname, lo, err := ps.lookupWalk(fid, wnames, resolve)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ps.stats.IncPath(fname)
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

func (ps *ProtSrv) RemoveFile(args np.Tremovefile, rets *np.Rremove) *np.Rerror {
	f, fname, lo, err := ps.lookupWalk(args.Fid, args.Wnames, args.Resolve)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "%v: RemoveFile %v %v %v", f.Pobj().Ctx().Uname(), f.Pobj().Path(), fname, args.Fid)
	return ps.removeObj(f.Pobj().Ctx(), lo, fname)
}

func (ps *ProtSrv) GetFile(args np.Tgetfile, rets *np.Rgetfile) *np.Rerror {
	if args.Count > np.MAXGETSET {
		return np.MkErr(np.TErrInval, "too large").Rerror()
	}
	f, fname, lo, i, err := ps.lookupWalkOpen(args.Fid, args.Wnames, args.Resolve, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("PROTSRV", "GetFile f %v args %v %v\n", f.Pobj().Ctx().Uname(), args, fname)
	rets.Data, err = i.Read(f.Pobj().Ctx(), args.Offset, args.Count, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	if err := lo.Close(f.Pobj().Ctx(), args.Mode); err != nil {
		return err.Rerror()
	}
	return nil
}

func (ps *ProtSrv) SetFile(args np.Tsetfile, rets *np.Rwrite) *np.Rerror {
	if np.Tsize(len(args.Data)) > np.MAXGETSET {
		return np.MkErr(np.TErrInval, "too large").Rerror()
	}
	f, fname, lo, i, err := ps.lookupWalkOpen(args.Fid, args.Wnames, args.Resolve, args.Mode)
	if err != nil {
		return err.Rerror()
	}

	db.DPrintf("PROTSRV", "SetFile f %v args %v %v\n", f.Pobj().Ctx().Uname(), args, fname)

	if args.Mode&np.OAPPEND == np.OAPPEND && args.Offset != np.NoOffset {
		return np.MkErr(np.TErrInval, "offset should be np.NoOffset").Rerror()

	}
	if args.Offset == np.NoOffset && args.Mode&np.OAPPEND != np.OAPPEND {
		return np.MkErr(np.TErrInval, "mode shouldbe OAPPEND").Rerror()

	}

	n, err := i.Write(f.Pobj().Ctx(), args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.Rerror()
	}

	if err := lo.Close(f.Pobj().Ctx(), args.Mode); err != nil {
		return err.Rerror()
	}
	rets.Count = n
	return nil
}

func (ps *ProtSrv) PutFile(args np.Tputfile, rets *np.Rwrite) *np.Rerror {
	if np.Tsize(len(args.Data)) > np.MAXGETSET {
		return np.MkErr(np.TErrInval, "too large").Rerror()
	}
	// walk to directory
	f, dname, lo, err := ps.lookupWalk(args.Fid, args.Wnames[0:len(args.Wnames)-1], false)
	if err != nil {
		return err.Rerror()
	}
	fname := append(f.Pobj().Path(), args.Wnames...)

	db.DPrintf("PROTSRV", "%v: PutFile o %v args %v (%v)\n", f.Pobj().Ctx().Uname(), f, args, dname)

	if !lo.Perm().IsDir() {
		return np.MkErr(np.TErrNotDir, dname).Rerror()
	}
	name := args.Wnames[len(args.Wnames)-1]
	dws, fws := ps.AcquireWatches(dname, name)
	defer ps.ReleaseWatches(dws, fws)

	// AcquireWatches() ensures that Create and Write execute atomically

	lo, err = ps.createObj(f.Pobj().Ctx(), lo.(fs.Dir), dws, fws, name, args.Perm, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	qid := ps.mkQid(lo.Perm(), lo.Path())
	f = ps.makeFid(f.Pobj().Ctx(), dname, name, lo, args.Perm.IsEphemeral(), qid)
	i, err := fs.Obj2File(lo, fname)
	if err != nil {
		return err.Rerror()
	}
	n, err := i.Write(f.Pobj().Ctx(), args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	err = lo.Close(f.Pobj().Ctx(), args.Mode)
	if err != nil {
		return err.Rerror()
	}
	rets.Count = n
	return nil
}

func (ps *ProtSrv) Snapshot() []byte {
	return ps.snapshot()
}
