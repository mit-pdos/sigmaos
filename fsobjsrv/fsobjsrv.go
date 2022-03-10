package fsobjsrv

import (
	"log"
	// "time"

	db "ulambda/debug"
	"ulambda/fences"
	"ulambda/fid"
	"ulambda/fs"
	"ulambda/fssrv"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/protsrv"
	"ulambda/stats"
	"ulambda/watch"
)

//
// There is one FsObjSrv per session, but they share the watch table,
// and stats.  Each session has its own fid table, ephemeral table,
// and lease table.
//

type FsObjSrv struct {
	fssrv *fssrv.FsServer
	wt    *watch.WatchTable // shared across sessions
	ft    *fidTable
	et    *ephemeralTable
	rft   *fences.RecentTable // shared across sessions
	stats *stats.Stats
	sid   np.Tsession
}

func MakeProtServer(s protsrv.FsServer, sid np.Tsession) protsrv.Protsrv {
	fos := &FsObjSrv{}
	srv := s.(*fssrv.FsServer)
	fos.fssrv = srv

	fos.ft = makeFidTable()
	fos.et = makeEphemeralTable()
	fos.wt = srv.GetWatchTable()
	fos.stats = srv.GetStats()
	fos.rft = srv.GetRecentFences()
	fos.sid = sid
	db.DLPrintf("NPOBJ", "MakeFsObjSrv -> %v", fos)
	return fos
}

func (fos *FsObjSrv) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (fos *FsObjSrv) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.MkErr(np.TErrNotSupported, "Auth").Rerror()
}

func (fos *FsObjSrv) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	db.DLPrintf("FSOBJ", "Attach %v\n", args.Uname)
	path := np.Split(args.Aname)
	root, ctx := fos.fssrv.AttachTree(args.Uname, args.Aname, fos.sid)
	tree := root.(fs.FsObj)
	if args.Aname != "" {
		os, rest, err := root.Lookup(ctx, path)
		if len(rest) > 0 || err != nil {
			return err.Rerror()
		}
		tree = os[len(os)-1]
	}
	fos.ft.Add(args.Fid, fid.MakeFidPath(path, tree, 0, ctx))
	rets.Qid = tree.(fs.FsObj).Qid()
	return nil
}

// Delete ephemeral files created on this session.
func (fos *FsObjSrv) Detach() {

	// Several threads maybe waiting in a sesscond. DeleteSess
	// will unblock them so that they can bail out.
	fos.fssrv.GetSessCondTable().DeleteSess(fos.sid)

	fos.ft.ClunkOpen()
	ephemeral := fos.et.Get()
	for o, f := range ephemeral {
		db.DLPrintf("FSOBJ", "Detach %v\n", f.Path())
		fos.removeObj(f.Ctx(), o, f.Path())
	}
}

func makeQids(os []fs.FsObj) []np.Tqid {
	var qids []np.Tqid
	for _, o := range os {
		qids = append(qids, o.Qid())
	}
	return qids
}

func (fos *FsObjSrv) lookupObj(ctx fs.CtxI, f *fid.Fid, names np.Path) ([]fs.FsObj, np.Path, *np.Err) {
	o := f.Obj()
	if !o.Perm().IsDir() {
		return nil, nil, np.MkErr(np.TErrNotDir, f.Path().Base())
	}
	d := o.(fs.Dir)
	return d.Lookup(ctx, names)
}

func (fos *FsObjSrv) lookupObjLast(ctx fs.CtxI, f *fid.Fid, names np.Path, resolve bool) (fs.FsObj, *np.Err) {
	os, _, err := fos.lookupObj(ctx, f, names)
	if err != nil {
		return nil, err
	}
	lo := os[len(os)-1]
	if lo.Perm().IsSymlink() && resolve {
		return nil, np.MkErr(np.TErrNotDir, names[len(names)-1])
	}
	return lo, nil
}

func (fos *FsObjSrv) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: Walk o %v args %v (%v)\n", f.Ctx().Uname(), f, args, len(args.Wnames))
	os, rest, err := fos.lookupObj(f.Ctx(), f, args.Wnames)
	if err != nil && !np.IsMaybeSpecialElem(err) {
		return err.Rerror()
	}
	// let the client decide what to do with rest
	n := len(args.Wnames) - len(rest)
	p := append(f.Path(), args.Wnames[:n]...)
	rets.Qids = makeQids(os)
	if len(os) == 0 { // cloning f into args.NewFid in ft
		os = append(os, f.Obj())
	}
	fos.ft.Add(args.NewFid, fid.MakeFidPath(p, os[len(os)-1], 0, f.Ctx()))
	return nil
}

func (fos *FsObjSrv) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: Clunk %v %v\n", f.Ctx().Uname(), args.Fid, f)
	if f.IsOpen() { // has the fid been opened?
		f.Obj().Close(f.Ctx(), f.Mode())
		f.Close()
	}
	fos.ft.Del(args.Fid)
	return nil
}

func (fos *FsObjSrv) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: f %v %v\n", f.Ctx().Uname(), f, args)

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

func (fos *FsObjSrv) WatchV(args np.Twatchv, rets *np.Ropen) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	o := f.Obj()
	p := f.Path()
	if len(args.Path) > 0 {
		p = append(p, args.Path...)
	}
	db.DLPrintf("FSOBJ", "%v: Watchv v %v %v\n", f.Ctx().Uname(), o.Version(), args)

	// get lock on watch entry for p, so that remove cannot remove
	// file before watch is set.
	ws := fos.wt.WatchLookupL(p)
	defer fos.wt.Release(ws)

	if o.Nlink() == 0 {
		return np.MkErr(np.TErrNotfound, f.Path()).Rerror()
	}
	if !np.VEq(args.Version, o.Version()) {

		return np.MkErr(np.TErrVersion, f.Path()).Rerror()
	}
	// time.Sleep(1000 * time.Nanosecond)

	err = ws.Watch(fos.sid)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) makeFid(ctx fs.CtxI, dir np.Path, name string, o fs.FsObj, eph bool) *fid.Fid {
	p := dir.Copy()
	nf := fid.MakeFidPath(append(p, name), o, 0, ctx)
	if eph {
		fos.et.Add(o, nf)
	}
	return nf
}

// Create name in dir. If OWATCH is set and name already exits, wait
// until another thread deletes it, and retry.
func (fos *FsObjSrv) createObj(ctx fs.CtxI, d fs.Dir, dws, fws *watch.Watch, name string, perm np.Tperm, mode np.Tmode) (fs.FsObj, *np.Err) {
	for {
		o1, err := d.Create(ctx, name, perm, mode)
		db.DLPrintf("FSOBJ", "%v: Create %v %v %v ephemeral %v\n", ctx.Uname(), name, o1, err, perm.IsEphemeral())
		if err == nil {
			fws.WakeupWatchL()
			dws.WakeupWatchL()
			return o1, nil
		} else {
			if mode&np.OWATCH == np.OWATCH && err.Code() == np.TErrExists {
				fws.Unlock()
				err := dws.Watch(fos.sid)
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

func (fos *FsObjSrv) AcquireWatches(dir np.Path, file string) (*watch.Watch, *watch.Watch) {
	dws := fos.wt.WatchLookupL(dir)
	fws := fos.wt.WatchLookupL(append(dir, file))
	return dws, fws
}

func (fos *FsObjSrv) ReleaseWatches(dws, fws *watch.Watch) {
	fos.wt.Release(dws)
	fos.wt.Release(fws)
}

func (fos *FsObjSrv) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: Create f %v\n", f.Ctx().Uname(), f)
	if err := fos.fssrv.Sess(fos.sid).CheckFences(f.Path()); err != nil {
		return err.Rerror()
	}
	o := f.Obj()
	names := np.Path{args.Name}
	if !o.Perm().IsDir() {
		return np.MkErr(np.TErrNotDir, f.Path()).Rerror()
	}
	d := o.(fs.Dir)
	dws, fws := fos.AcquireWatches(f.Path(), names[0])
	defer fos.ReleaseWatches(dws, fws)

	o1, err := fos.createObj(f.Ctx(), d, dws, fws, names[0], args.Perm, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	nf := fos.makeFid(f.Ctx(), f.Path(), names[0], o1, args.Perm.IsEphemeral())
	fos.rft.UpdateSeqno(nf.Path())
	fos.ft.Add(args.Fid, nf)
	rets.Qid = o1.Qid()
	return nil
}

func (fos *FsObjSrv) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (fos *FsObjSrv) lookupFence(fid np.Tfid) (*fid.Fid, *np.Err) {
	f, err := fos.ft.Lookup(fid)
	if err != nil {
		return nil, err
	}
	if err := fos.fssrv.Sess(fos.sid).CheckFences(f.Path()); err != nil {
		return nil, err
	}
	return f, nil
}

func (fos *FsObjSrv) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	err = f.Read(args.Offset, args.Count, np.NoV, rets)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	f, err := fos.lookupFence(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	rets.Count, err = f.Write(args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) removeObj(ctx fs.CtxI, o fs.FsObj, path np.Path) *np.Rerror {
	// lock watch entry to make WatchV and Remove interact
	// correctly

	dws := fos.wt.WatchLookupL(path.Dir())
	fws := fos.wt.WatchLookupL(path)
	defer fos.wt.Release(dws)
	defer fos.wt.Release(fws)

	fos.stats.Path(path)

	db.DLPrintf("FSOBJ", "%v: removeObj %v in %v\n", ctx.Uname(), path, path.Dir())

	// Call before Remove(), because after remove o's underlying
	// object may not exist anymore.
	ephemeral := o.Perm().IsEphemeral()
	err := o.Parent().Remove(ctx, path[len(path)-1])
	if err != nil {
		return err.Rerror()
	}
	err = o.Unlink(ctx)
	if err != nil {
		return err.Rerror()
	}

	fws.WakeupWatchL()
	dws.WakeupWatchL()

	if ephemeral {
		fos.et.Del(o)
	}
	return nil
}

// Remove for backwards compatability; SigmaOS uses RemoveFile (see
// below) instead of Remove, but proxy will use it.
func (fos *FsObjSrv) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	f, err := fos.lookupFence(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: Remove %v\n", f.Ctx().Uname(), f.Path())
	return fos.removeObj(f.Ctx(), f.Obj(), f.Path())
}

func (fos *FsObjSrv) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: Stat %v\n", f.Ctx().Uname(), f)
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

func (fos *FsObjSrv) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	f, err := fos.lookupFence(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: Wstat %v %v\n", f.Ctx().Uname(), f, args)
	o := f.Obj()
	if args.Stat.Name != "" {
		// update Name atomically with rename

		if err := fos.fssrv.Sess(fos.sid).CheckFences(f.PathDir()); err != nil {
			return err.Rerror()
		}

		dst := f.PathDir().Copy().AppendPath(np.Split(args.Stat.Name))

		dws := fos.wt.WatchLookupL(f.PathDir())
		defer fos.wt.Release(dws)
		sws := fos.wt.WatchLookupL(f.Path())
		defer fos.wt.Release(sws)
		tws := fos.wt.WatchLookupL(dst)
		defer fos.wt.Release(tws)

		err := o.Parent().Rename(f.Ctx(), f.PathBase(), args.Stat.Name)
		if err != nil {
			return err.Rerror()
		}
		fos.rft.UpdateSeqno(dst)
		tws.WakeupWatchL() // trigger create watch
		sws.WakeupWatchL() // trigger remove watch
		dws.WakeupWatchL() // trigger dir watch
		f.SetPath(dst)
	}
	// XXX ignore other Wstat for now
	return nil
}

func lockOrder(d1 fs.FsObj, oldf *fid.Fid, d2 fs.FsObj, newf *fid.Fid) (*fid.Fid, *fid.Fid) {
	if d1.Inum() < d2.Inum() {
		return oldf, newf
	} else if d1.Inum() == d2.Inum() { // would have used wstat instead of renameat
		log.Fatalf("FATAL lockOrder")
		return oldf, newf
	} else {
		return newf, oldf
	}
}

func (fos *FsObjSrv) Renameat(args np.Trenameat, rets *np.Rrenameat) *np.Rerror {
	oldf, err := fos.ft.Lookup(args.OldFid)
	if err != nil {
		return err.Rerror()
	}
	newf, err := fos.ft.Lookup(args.NewFid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: renameat %v %v %v\n", oldf.Ctx().Uname(), oldf, newf, args)
	oo := oldf.Obj()
	no := newf.Obj()
	switch d1 := oo.(type) {
	case fs.Dir:
		d2, ok := no.(fs.Dir)
		if !ok {
			return np.MkErr(np.TErrNotDir, newf.Path()).Rerror()
		}
		if oo.Inum() == no.Inum() {
			return np.MkErr(np.TErrInval, newf.Path()).Rerror()
		}

		if err := fos.fssrv.Sess(fos.sid).CheckFences(oldf.PathDir()); err != nil {
			return err.Rerror()
		}
		if err := fos.fssrv.Sess(fos.sid).CheckFences(newf.PathDir()); err != nil {
			return err.Rerror()
		}

		f1, f2 := lockOrder(oo, oldf, no, newf)
		d1ws := fos.wt.WatchLookupL(f1.Path())
		d2ws := fos.wt.WatchLookupL(f2.Path())
		defer fos.wt.Release(d1ws)
		defer fos.wt.Release(d2ws)

		src := oldf.Path().Copy().Append(args.OldName)
		dst := newf.Path().Copy().Append(args.NewName)
		srcws := fos.wt.WatchLookupL(src)
		dstws := fos.wt.WatchLookupL(dst)
		defer fos.wt.Release(srcws)
		defer fos.wt.Release(dstws)

		err := d1.Renameat(oldf.Ctx(), args.OldName, d2, args.NewName)
		if err != nil {

			return err.Rerror()
		}
		fos.rft.UpdateSeqno(dst)
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

func (fos *FsObjSrv) lookupWalkFence(fid np.Tfid, wnames np.Path, resolve bool) (*fid.Fid, np.Path, fs.FsObj, *np.Err) {
	f, err := fos.ft.Lookup(fid)
	if err != nil {
		return nil, nil, nil, err
	}
	lo := f.Obj()
	fname := append(f.Path(), wnames...)
	if len(wnames) > 0 {
		lo, err = fos.lookupObjLast(f.Ctx(), f, wnames, resolve)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	if err := fos.fssrv.Sess(fos.sid).CheckFences(fname); err != nil {
		return nil, nil, nil, err
	}
	return f, fname, lo, nil
}

func (fos *FsObjSrv) lookupWalkFenceOpen(fid np.Tfid, wnames np.Path, resolve bool, mode np.Tmode) (*fid.Fid, np.Path, fs.File, *np.Err) {
	f, fname, lo, err := fos.lookupWalkFence(fid, wnames, resolve)
	if err != nil {
		return nil, nil, nil, err
	}
	fos.stats.Path(fname)
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

func (fos *FsObjSrv) RemoveFile(args np.Tremovefile, rets *np.Rremove) *np.Rerror {
	f, fname, lo, err := fos.lookupWalkFence(args.Fid, args.Wnames, args.Resolve)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: RemoveFile %v\n", f.Ctx().Uname(), fname)
	return fos.removeObj(f.Ctx(), lo, fname)
}

func (fos *FsObjSrv) GetFile(args np.Tgetfile, rets *np.Rgetfile) *np.Rerror {
	if args.Count > np.MAXGETSET {
		return np.MkErr(np.TErrInval, "too large").Rerror()
	}
	f, fname, i, err := fos.lookupWalkFenceOpen(args.Fid, args.Wnames, args.Resolve, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "GetFile f %v args %v %v\n", f.Ctx(), args, fname)
	rets.Data, err = i.Read(f.Ctx(), args.Offset, args.Count, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	if err := f.Obj().Close(f.Ctx(), args.Mode); err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) SetFile(args np.Tsetfile, rets *np.Rwrite) *np.Rerror {
	if np.Tsize(len(args.Data)) > np.MAXGETSET {
		return np.MkErr(np.TErrInval, "too large").Rerror()
	}
	f, fname, i, err := fos.lookupWalkFenceOpen(args.Fid, args.Wnames, args.Resolve, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	n, err := i.Write(f.Ctx(), args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	if err := f.Obj().Close(f.Ctx(), args.Mode); err != nil {
		return err.Rerror()
	}
	rets.Count = n
	fos.rft.UpdateSeqno(fname)
	return nil
}

func (fos *FsObjSrv) PutFile(args np.Tputfile, rets *np.Rwrite) *np.Rerror {
	if np.Tsize(len(args.Data)) > np.MAXGETSET {
		return np.MkErr(np.TErrInval, "too large").Rerror()
	}
	// walk to directory
	f, dname, lo, err := fos.lookupWalkFence(args.Fid, args.Wnames[0:len(args.Wnames)-1], false)
	if err != nil {
		return err.Rerror()
	}
	fname := append(f.Path(), args.Wnames...)

	db.DLPrintf("FSOBJ", "%v: PutFile o %v args %v (%v)\n", f.Ctx().Uname(), f, args, dname)

	if !lo.Perm().IsDir() {
		return np.MkErr(np.TErrNotDir, dname).Rerror()
	}
	name := args.Wnames[len(args.Wnames)-1]
	dws, fws := fos.AcquireWatches(dname, name)
	defer fos.ReleaseWatches(dws, fws)

	lo, err = fos.createObj(f.Ctx(), lo.(fs.Dir), dws, fws, name, args.Perm, args.Mode)
	if err != nil {
		return err.Rerror()
	}
	f = fos.makeFid(f.Ctx(), dname, name, lo, args.Perm.IsEphemeral())
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
	fos.rft.UpdateSeqno(fname)
	return nil
}

//
// Fences
//

func (fos *FsObjSrv) MkFence(args np.Tmkfence, rets *np.Rmkfence) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	rets.Fence = fos.rft.MkFence(f.Path())
	db.DLPrintf("FSOBJ", "%v: mkfence f %v -> %v\n", f.Ctx().Uname(), f.Path(), rets.Fence)
	return nil
}

func (fos *FsObjSrv) RmFence(args np.Trmfence, rets *np.Ropen) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: rmfence %v %v\n", f.Ctx().Uname(), f.Path(), args.Fence)
	err = fos.rft.RmFence(args.Fence)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) RegFence(args np.Tregfence, rets *np.Ropen) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: RegFence %v %v\n", f.Ctx().Uname(), f.Path(), args)
	err = fos.rft.UpdateFence(args.Fence)
	if err != nil {
		log.Printf("%v: Fence %v %v err %v\n", proc.GetName(), fos.sid, args, err)
		return err.Rerror()
	}
	// Fence was present in recent fences table and not stale, or
	// was not present. Now mark that all ops on this sess must be
	// checked against the most recently-seen fence in rft.
	// Another sess may register a more recent fence in rft in the
	// future, and then ops on this session should fail.  Fence
	// may be called many times on sess, because client may
	// register a more recent fence.
	fos.fssrv.Sess(fos.sid).Fence(f.Path(), args.Fence)
	return nil
}

func (fos *FsObjSrv) UnFence(args np.Tunfence, rets *np.Ropen) *np.Rerror {
	f, err := fos.ft.Lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("FSOBJ", "%v: Unfence %v %v\n", f.Ctx().Uname(), f.Path(), args)
	err = fos.fssrv.Sess(fos.sid).Unfence(f.Path(), args.Fence.FenceId)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) Snapshot() []byte {
	return fos.snapshot()
}
