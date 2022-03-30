package protsrv

import (
	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/sesssrv"
	"ulambda/stats"
	"ulambda/watch"
)

//
// There is one protsrv per session, but they share the watch table,
// and stats across sessions.  Each session has its own fid table,
// ephemeral table, and lease table.
//

type ProtSrv struct {
	ssrv  *sesssrv.SessSrv
	wt    *watch.WatchTable // shared across sessions
	ft    *fidTable
	et    *ephemeralTable
	stats *stats.Stats
	sid   np.Tsession
}

func MakeProtServer(s np.FsServer, sid np.Tsession) np.Protsrv {
	ps := &ProtSrv{}
	srv := s.(*sesssrv.SessSrv)
	ps.ssrv = srv

	ps.ft = makeFidTable()
	ps.et = makeEphemeralTable()
	ps.wt = srv.GetWatchTable()
	ps.stats = srv.GetStats()
	ps.sid = sid
	db.DPrintf("NPOBJ", "MakeProtSrv -> %v", ps)
	return ps
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
	db.DPrintf("FSOBJ", "Attach %v\n", args)
	path := np.Split(args.Aname)
	root, ctx := ps.ssrv.AttachTree(args.Uname, args.Aname, ps.sid)
	tree := root.(fs.FsObj)
	qid := tree.(fs.FsObj).Qid()
	if args.Aname != "" {
		qids, lo, rest, err := root.Lookup(ctx, path)
		if len(rest) > 0 || err != nil {
			return err.Rerror()
		}
		tree = lo
		qid = qids[len(qids)-1]
	}
	ps.ft.Add(args.Fid, fid.MakeFidPath(path, tree, 0, ctx, qid))
	rets.Qid = qid
	return nil
}

// Delete ephemeral files created on this session.
func (ps *ProtSrv) Detach() {
	db.DPrintf("FSOBJ", "Detach %v eph %v\n", ps.sid, ps.et.Get())

	// Several threads maybe waiting in a sesscond. DeleteSess
	// will unblock them so that they can bail out.
	ps.ssrv.GetSessCondTable().DeleteSess(ps.sid)

	ps.ft.ClunkOpen()
	ephemeral := ps.et.Get()
	for o, f := range ephemeral {
		db.DPrintf("FSOBJ", "Detach %v\n", f.Path())
		ps.removeObj(f.Ctx(), o, f.Path())
	}
}

func makeQids(os []fs.FsObj) []np.Tqid {
	var qids []np.Tqid
	for _, o := range os {
		qids = append(qids, o.Qid())
	}
	return qids
}

func (ps *ProtSrv) lookupObj(ctx fs.CtxI, f *fid.Fid, names np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	o := f.Obj()
	if !o.Perm().IsDir() {
		return nil, nil, nil, np.MkErr(np.TErrNotDir, f.Path().Base())
	}
	d := o.(fs.Dir)
	return d.Lookup(ctx, names)
}

func (ps *ProtSrv) lookupObjLast(ctx fs.CtxI, f *fid.Fid, names np.Path, resolve bool) (fs.FsObj, *np.Err) {
	_, lo, _, err := ps.lookupObj(ctx, f, names)
	if err != nil {
		return nil, err
	}
	if lo.Perm().IsSymlink() && resolve {
		return nil, np.MkErr(np.TErrNotDir, names[len(names)-1])
	}
	return lo, nil
}

func (ps *ProtSrv) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("FSOBJ", "%v: Walk o %v args %v (%v)\n", f.Ctx().Uname(), f, args, len(args.Wnames))
	qids, lo, rest, err := ps.lookupObj(f.Ctx(), f, args.Wnames)
	if err != nil && !np.IsMaybeSpecialElem(err) {
		return err.Rerror()
	}
	// let the client decide what to do with rest
	n := len(args.Wnames) - len(rest)
	p := append(f.Path(), args.Wnames[:n]...)
	rets.Qids = qids
	qid := f.Obj().Qid()
	if len(qids) == 0 { // cloning f into args.NewFid in ft
		lo = f.Obj()
	} else {
		qid = qids[len(qids)-1]
	}
	ps.ft.Add(args.NewFid, fid.MakeFidPath(p, lo, 0, f.Ctx(), qid))
	return nil
}

func (ps *ProtSrv) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("FSOBJ", "%v: Clunk %v %v\n", f.Ctx().Uname(), args.Fid, f)
	if f.IsOpen() { // has the fid been opened?
		f.Obj().Close(f.Ctx(), f.Mode())
		f.Close()
	}
	ps.ft.Del(args.Fid)
	return nil
}

func (ps *ProtSrv) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("FSOBJ", "%v: Open f %v %v\n", f.Ctx().Uname(), f, args)

	o := f.Obj()
	no, r := o.Open(f.Ctx(), args.Mode)
	if r != nil {
		return r.Rerror()
	}
	f.SetMode(args.Mode)
	if no != nil {
		f.SetObj(no)
		rets.Qid = no.Qid()
	} else {
		rets.Qid = o.Qid()
	}
	return nil
}

func (ps *ProtSrv) Watch(args np.Twatch, rets *np.Ropen) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	o := f.Obj()
	p := f.Path()

	db.DPrintf("FSOBJ", "%v: Watch %v v %v %v\n", f.Ctx().Uname(), f.Path(), o.Qid(), args)

	// get lock on watch entry for p, so that remove cannot remove
	// file before watch is set.
	ws := ps.wt.WatchLookupL(p)
	defer ps.wt.Release(ws)

	if !np.VEq(f.Qid().Version, o.Qid().Version) {
		return np.MkErr(np.TErrVersion, o.Qid()).Rerror()
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
	nf := fid.MakeFidPath(append(p, name), o, 0, ctx, qid)
	if eph {
		ps.et.Add(o, nf)
	}
	return nf
}

// Create name in dir. If OWATCH is set and name already exits, wait
// until another thread deletes it, and retry.
func (ps *ProtSrv) createObj(ctx fs.CtxI, d fs.Dir, dws, fws *watch.Watch, name string, perm np.Tperm, mode np.Tmode) (fs.FsObj, *np.Err) {
	for {
		o1, err := d.Create(ctx, name, perm, mode)
		db.DPrintf("FSOBJ", "%v: Create %v %v %v ephemeral %v %v\n", ctx.Uname(), name, o1, err, perm.IsEphemeral(), ps.sid)
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
	db.DPrintf("FSOBJ", "%v: Create f %v\n", f.Ctx().Uname(), f)
	o := f.Obj()
	names := np.Path{args.Name}
	if !o.Perm().IsDir() {
		return np.MkErr(np.TErrNotDir, f.Path()).Rerror()
	}
	d := o.(fs.Dir)
	dws, fws := ps.AcquireWatches(f.Path(), names[0])
	defer ps.ReleaseWatches(dws, fws)

	o1, err := ps.createObj(f.Ctx(), d, dws, fws, names[0], args.Perm, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	nf := ps.makeFid(f.Ctx(), f.Path(), names[0], o1, args.Perm.IsEphemeral(), o1.Qid())
	ps.ft.Add(args.Fid, nf)
	rets.Qid = o1.Qid()
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
	db.DPrintf("FSOBJ", "%v: Read f %v args %v\n", f.Ctx().Uname(), f, args)
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
	db.DPrintf("FSOBJ", "%v: Read1 f %v args %v\n", f.Ctx().Uname(), f, args)
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
	return nil
}

func (ps *ProtSrv) WriteV(args np.TwriteV, rets *np.Rwrite) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("FSOBJ", "%v: Writev1 %v %v\n", f.Ctx().Uname(), f.Path(), args)
	rets.Count, err = f.Write(args.Offset, args.Data, args.Version)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (ps *ProtSrv) removeObj(ctx fs.CtxI, o fs.FsObj, path np.Path) *np.Rerror {
	// lock watch entry to make WatchV and Remove interact
	// correctly

	dws := ps.wt.WatchLookupL(path.Dir())
	fws := ps.wt.WatchLookupL(path)
	defer ps.wt.Release(dws)
	defer ps.wt.Release(fws)

	ps.stats.Path(path)

	db.DPrintf("FSOBJ", "%v: removeObj %v in %v\n", ctx.Uname(), path, path.Dir())

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	ephemeral := o.Perm().IsEphemeral()
	err := o.Parent().Remove(ctx, path[len(path)-1])
	if err != nil {
		return err.Rerror()
	}

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
	db.DPrintf("FSOBJ", "%v: Remove %v\n", f.Ctx().Uname(), f.Path())
	return ps.removeObj(f.Ctx(), f.Obj(), f.Path())
}

func (ps *ProtSrv) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	f, err := ps.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("FSOBJ", "%v: Stat %v\n", f.Ctx().Uname(), f)
	o := f.Obj()
	st, r := o.Stat(f.Ctx())
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
	db.DPrintf("FSOBJ", "%v: Wstat %v %v\n", f.Ctx().Uname(), f, args)
	o := f.Obj()
	if args.Stat.Name != "" {
		// update Name atomically with rename

		dst := f.Path().Dir().Copy().AppendPath(np.Split(args.Stat.Name))

		dws, sws := ps.AcquireWatches(f.Path().Dir(), f.Path().Base())
		defer ps.ReleaseWatches(dws, sws)
		tws := ps.wt.WatchLookupL(dst)
		defer ps.wt.Release(tws)

		err := o.Parent().Rename(f.Ctx(), f.Path().Base(), args.Stat.Name)
		if err != nil {
			return err.Rerror()
		}
		tws.WakeupWatchL() // trigger create watch
		sws.WakeupWatchL() // trigger remove watch
		dws.WakeupWatchL() // trigger dir watch
		f.SetPath(dst)
	}
	// XXX ignore other Wstat for now
	return nil
}

// d1 first?
func lockOrder(d1 fs.FsObj, d2 fs.FsObj) bool {
	if d1.Qid().Path < d2.Qid().Path {
		return true
	} else if d1.Qid().Path == d2.Qid().Path { // would have used wstat instead of renameat
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
	db.DPrintf("FSOBJ", "%v: renameat %v %v %v\n", oldf.Ctx().Uname(), oldf, newf, args)
	oo := oldf.Obj()
	no := newf.Obj()
	switch d1 := oo.(type) {
	case fs.Dir:
		d2, ok := no.(fs.Dir)
		if !ok {
			return np.MkErr(np.TErrNotDir, newf.Path()).Rerror()
		}
		if oo.Qid().Path == no.Qid().Path {
			return np.MkErr(np.TErrInval, newf.Path()).Rerror()
		}

		var d1ws, d2ws, srcws, dstws *watch.Watch
		if srcfirst := lockOrder(oo, no); srcfirst {
			d1ws, srcws = ps.AcquireWatches(oldf.Path(), args.OldName)
			d2ws, dstws = ps.AcquireWatches(newf.Path(), args.NewName)
		} else {
			d2ws, dstws = ps.AcquireWatches(newf.Path(), args.NewName)
			d1ws, srcws = ps.AcquireWatches(oldf.Path(), args.OldName)
		}
		defer ps.ReleaseWatches(d1ws, srcws)
		defer ps.ReleaseWatches(d2ws, dstws)

		err := d1.Renameat(oldf.Ctx(), args.OldName, d2, args.NewName)
		if err != nil {
			return err.Rerror()
		}
		dstws.WakeupWatchL() // trigger create watch
		srcws.WakeupWatchL() // trigger remove watch
		d1ws.WakeupWatchL()  // trigger one dir watch
		d2ws.WakeupWatchL()  // trigger the other dir watch
	default:
		return np.MkErr(np.TErrNotDir, oldf.Path()).Rerror()
	}
	return nil
}

//
// Requests that combine walk, open, and do operation in a single RPC,
// which also avoids clunking. They may fail because args.Wnames may
// contains a special path element; in that, case the client must walk
// args.Wnames.
//

func (ps *ProtSrv) lookupWalk(fid np.Tfid, wnames np.Path, resolve bool) (*fid.Fid, np.Path, fs.FsObj, *np.Err) {
	f, err := ps.ft.Lookup(fid)
	if err != nil {
		return nil, nil, nil, err
	}
	lo := f.Obj()
	fname := append(f.Path(), wnames...)
	if len(wnames) > 0 {
		lo, err = ps.lookupObjLast(f.Ctx(), f, wnames, resolve)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return f, fname, lo, nil
}

func (ps *ProtSrv) lookupWalkOpen(fid np.Tfid, wnames np.Path, resolve bool, mode np.Tmode) (*fid.Fid, np.Path, fs.File, *np.Err) {
	f, fname, lo, err := ps.lookupWalk(fid, wnames, resolve)
	if err != nil {
		return nil, nil, nil, err
	}
	ps.stats.Path(fname)
	no, err := lo.Open(f.Ctx(), mode)
	if err != nil {
		return nil, nil, nil, err
	}
	if no != nil {
		lo = no
	}
	i, err := fs.Obj2File(lo, fname)
	if err != nil {
		lo.Close(f.Ctx(), mode)
		return nil, nil, nil, err
	}
	return f, fname, i, nil
}

func (ps *ProtSrv) RemoveFile(args np.Tremovefile, rets *np.Rremove) *np.Rerror {
	f, fname, lo, err := ps.lookupWalk(args.Fid, args.Wnames, args.Resolve)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("FSOBJ", "%v: RemoveFile %v\n", f.Ctx().Uname(), fname)
	return ps.removeObj(f.Ctx(), lo, fname)
}

func (ps *ProtSrv) GetFile(args np.Tgetfile, rets *np.Rgetfile) *np.Rerror {
	if args.Count > np.MAXGETSET {
		return np.MkErr(np.TErrInval, "too large").Rerror()
	}
	f, fname, i, err := ps.lookupWalkOpen(args.Fid, args.Wnames, args.Resolve, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	db.DPrintf("FSOBJ", "GetFile f %v args %v %v\n", f.Ctx().Uname(), args, fname)
	rets.Data, err = i.Read(f.Ctx(), args.Offset, args.Count, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	if err := f.Obj().Close(f.Ctx(), args.Mode); err != nil {
		return err.Rerror()
	}
	return nil
}

func (ps *ProtSrv) SetFile(args np.Tsetfile, rets *np.Rwrite) *np.Rerror {
	if np.Tsize(len(args.Data)) > np.MAXGETSET {
		return np.MkErr(np.TErrInval, "too large").Rerror()
	}
	f, fname, i, err := ps.lookupWalkOpen(args.Fid, args.Wnames, args.Resolve, args.Mode)
	if err != nil {
		return err.Rerror()
	}

	db.DPrintf("FSOBJ", "SetFile f %v args %v %v\n", f.Ctx().Uname(), args, fname)

	if args.Mode&np.OAPPEND == np.OAPPEND && args.Offset != np.NoOffset {
		return np.MkErr(np.TErrInval, "offset should be np.NoOffset").Rerror()

	}
	if args.Offset == np.NoOffset && args.Mode&np.OAPPEND != np.OAPPEND {
		return np.MkErr(np.TErrInval, "mode shouldbe OAPPEND").Rerror()

	}

	n, err := i.Write(f.Ctx(), args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.Rerror()
	}

	if err := f.Obj().Close(f.Ctx(), args.Mode); err != nil {
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
	fname := append(f.Path(), args.Wnames...)

	db.DPrintf("FSOBJ", "%v: PutFile o %v args %v (%v)\n", f.Ctx().Uname(), f, args, dname)

	if !lo.Perm().IsDir() {
		return np.MkErr(np.TErrNotDir, dname).Rerror()
	}
	name := args.Wnames[len(args.Wnames)-1]
	dws, fws := ps.AcquireWatches(dname, name)
	defer ps.ReleaseWatches(dws, fws)

	lo, err = ps.createObj(f.Ctx(), lo.(fs.Dir), dws, fws, name, args.Perm, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	f = ps.makeFid(f.Ctx(), dname, name, lo, args.Perm.IsEphemeral(), lo.Qid())
	i, err := fs.Obj2File(lo, fname)
	if err != nil {
		return err.Rerror()
	}
	n, err := i.Write(f.Ctx(), args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	err = lo.Close(f.Ctx(), args.Mode)
	if err != nil {
		return err.Rerror()
	}
	rets.Count = n
	return nil
}

func (ps *ProtSrv) Snapshot() []byte {
	return ps.snapshot()
}
