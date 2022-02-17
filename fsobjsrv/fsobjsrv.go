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

func (fos *FsObjSrv) lookup(fid np.Tfid) (*fid.Fid, *np.Err) {
	f, ok := fos.ft.Lookup(fid)
	if !ok {
		return nil, np.MkErr(np.TErrUnknownfid, fid)
	}
	return f, nil
}

func (fos *FsObjSrv) watch(ws *watch.Watch, sess np.Tsession) *np.Err {
	err := ws.Watch(sess)
	if err != nil {
		return err
	}
	return nil
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
	// log.Printf("%v: Attach %v\n", proc.GetName(), args.Uname)
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

	// log.Printf("%v: %v Clunkopen: %v\n", proc.GetName(), fos.sid, fos.ft.fids)
	fos.ft.ClunkOpen()
	ephemeral := fos.et.Get()
	db.DLPrintf("9POBJ", "Detach %v %v\n", fos.sid, ephemeral)
	// log.Printf("%v detach %v ephemeral %v\n", proc.GetName(), fos.sid, ephemeral)
	for o, f := range ephemeral {
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

func (fos *FsObjSrv) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "Walk o %v args %v (%v)\n", f, args, len(args.Wnames))
	if len(args.Wnames) == 0 { // clone args.Fid?
		o := f.Obj()
		fos.ft.Add(args.NewFid, fid.MakeFidPath(f.Path(), o, 0, f.Ctx()))
	} else {
		o := f.Obj()
		if !o.Perm().IsDir() {
			return np.MkErr(np.TErrNotDir, np.Join(f.Path())).Rerror()
		}
		d := o.(fs.Dir)
		os, rest, err := d.Lookup(f.Ctx(), args.Wnames)
		if err != nil && !np.IsMaybeSpecialElem(err) {
			return err.Rerror()
		}

		// let the client decide what to do with rest
		n := len(args.Wnames) - len(rest)
		p := append(f.Path(), args.Wnames[:n]...)

		if len(os) > 0 {
			lo := os[len(os)-1]
			fos.ft.Add(args.NewFid, fid.MakeFidPath(p, lo, 0, f.Ctx()))
		}
		rets.Qids = makeQids(os)
	}
	return nil
}

func (fos *FsObjSrv) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	db.DLPrintf("9POBJ", "Clunk %v\n", args)
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	o := f.Obj()
	if f.IsOpen() { // has the fid been opened?
		o.Close(f.Ctx(), f.Mode())
		f.Close()
	}
	fos.ft.Del(args.Fid)
	return nil
}

func (fos *FsObjSrv) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	db.DLPrintf("9POBJ", "Open %v\n", args)
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "f %v\n", f)

	o := f.Obj()
	// log.Printf("%v: %v open %v mode %v\n", proc.GetName(), f.Ctx().Uname(), f.Path(), args.Mode)
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
	db.DLPrintf("9POBJ", "Watchv %v\n", args)

	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	o := f.Obj()
	p := f.Path()
	if len(args.Path) > 0 {
		p = append(p, args.Path...)
	}

	// get lock on watch entry for p, so that remove cannot remove
	// file before watch is set.
	ws := fos.wt.WatchLookupL(p)
	defer fos.wt.Release(ws)

	if o.Nlink() == 0 {
		return np.MkErr(np.TErrNotfound, np.Join(f.Path())).Rerror()
	}
	if !np.VEq(args.Version, o.Version()) {

		return np.MkErr(np.TErrVersion, np.Join(f.Path())).Rerror()
	}
	// time.Sleep(1000 * time.Nanosecond)

	err = fos.watch(ws, fos.sid)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) makeFid(ctx fs.CtxI, dir []string, name string, o fs.FsObj, eph bool) *fid.Fid {
	p := np.Copy(dir)
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

		//log.Printf("%v %v %v Create %v %v %v ephemeral %v\n", proc.GetName(), fos.sid, ctx.Uname(), name, o1, err, perm.IsEphemeral())

		db.DLPrintf("9POBJ", "Create %v %v %v ephemeral %v\n", name, o1, err, perm.IsEphemeral())
		if err == nil {
			fws.WakeupWatchL()
			dws.WakeupWatchL()
			return o1, nil
		} else {
			if mode&np.OWATCH == np.OWATCH && err.Code() == np.TErrExists {
				fws.Unlock()
				err := fos.watch(dws, fos.sid)
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

func (fos *FsObjSrv) AcquireWatches(dir []string, file string) (*watch.Watch, *watch.Watch) {
	dws := fos.wt.WatchLookupL(dir)
	fws := fos.wt.WatchLookupL(append(dir, file))
	return dws, fws
}

func (fos *FsObjSrv) ReleaseWatches(dws, fws *watch.Watch) {
	fos.wt.Release(dws)
	fos.wt.Release(fws)
}

func (fos *FsObjSrv) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	db.DLPrintf("9POBJ", "Create %v\n", args)
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "Create f %v\n", f)
	if err := fos.fssrv.Sess(fos.sid).CheckFences(f.Path()); err != nil {
		log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), f.Path(), err)
		return err.Rerror()
	}
	o := f.Obj()
	names := []string{args.Name}
	if !o.Perm().IsDir() {
		return np.MkErr(np.TErrNotDir, np.Join(f.Path())).Rerror()
	}
	d := o.(fs.Dir)
	dws, fws := fos.AcquireWatches(f.Path(), names[0])
	defer fos.ReleaseWatches(dws, fws)

	o1, err := fos.createObj(f.Ctx(), d, dws, fws, names[0], args.Perm, args.Mode)
	if err != nil {
		//log.Printf("%v %v createObj %v err %v\n", proc.GetName(), f.Ctx().Uname(), names[0], r)
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

func (fos *FsObjSrv) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	db.DLPrintf("9POBJ", "Read %v\n", args)
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	if err := fos.fssrv.Sess(fos.sid).CheckFences(f.Path()); err != nil {
		log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), f.Path(), err)
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "ReadFid %v %v\n", args, f)
	err = f.Read(args.Offset, args.Count, np.NoV, rets)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	db.DLPrintf("9POBJ", "Write %v\n", args)
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	if err := fos.fssrv.Sess(fos.sid).CheckFences(f.Path()); err != nil {
		log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), f.Path(), err)
		return err.Rerror()
	}
	rets.Count, err = f.Write(args.Offset, args.Data, np.NoV)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

// XXX make .exit a reserved name
func isExit(path []string) bool {
	return len(path) == 1 && path[0] == ".exit"
}

func (fos *FsObjSrv) removeObj(ctx fs.CtxI, o fs.FsObj, path []string) *np.Rerror {
	// lock watch entry to make WatchV and Remove interact
	// correctly
	dws := fos.wt.WatchLookupL(np.Dir(path))
	fws := fos.wt.WatchLookupL(path)
	defer fos.wt.Release(dws)
	defer fos.wt.Release(fws)

	fos.stats.Path(path)

	// log.Printf("%v: %v remove %v in %v\n", proc.GetName(), ctx.Uname(), path, np.Dir(path))

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

	if o.Perm().IsEphemeral() {
		// log.Printf("%v del %v %v ephemeral %v\n", proc.GetName(), path, fos.sid, fos.et.ephemeral)
		fos.et.Del(o)
	}
	return nil
}

func (fos *FsObjSrv) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	log.Printf("%v: %v remove %v\n", proc.GetName(), f.Ctx().Uname(), f.Path())
	if err := fos.fssrv.Sess(fos.sid).CheckFences(f.Path()); err != nil {
		log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), f.Path(), err)
		return err.Rerror()
	}
	o := f.Obj()
	if isExit(f.Path()) {
		db.DLPrintf("9POBJ", "Done\n")
		fos.fssrv.Done()
		return nil
	}
	return fos.removeObj(f.Ctx(), o, f.Path())
}

func (fos *FsObjSrv) lookupObj(ctx fs.CtxI, f *fid.Fid, names []string) (fs.FsObj, *np.Err) {
	o := f.Obj()
	if !o.Perm().IsDir() {
		return nil, np.MkErr(np.TErrNotDir, np.Base(f.Path()))
	}
	d := o.(fs.Dir)
	os, _, err := d.Lookup(ctx, names)
	if err != nil {
		return nil, err
	}
	return os[len(os)-1], nil
}

// RemoveFile is Remove() but args.Wnames may contain a symlink that
// hasn't been walked. If so, RemoveFile() will not succeed looking up
// args.Wnames, and caller should first walk the pathname.
func (fos *FsObjSrv) RemoveFile(args np.Tremovefile, rets *np.Rremove) *np.Rerror {
	var err *np.Err
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	// log.Printf("%v: %v removefile %v %v\n", proc.GetName(), f.Ctx().Uname(), f.Path(), args.Wnames)
	o := f.Obj()
	fname := append(f.Path(), args.Wnames[0:len(args.Wnames)]...)
	if isExit(fname) {
		db.DLPrintf("9POBJ", "Done\n")
		fos.fssrv.Done()
		return nil
	}
	if err := fos.fssrv.Sess(fos.sid).CheckFences(fname); err != nil {
		log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), f.Path(), err)
		return err.Rerror()
	}

	lo := o
	if len(args.Wnames) > 0 {
		lo, err = fos.lookupObj(f.Ctx(), f, args.Wnames)
		if err != nil {
			return err.Rerror()
		}
	}
	return fos.removeObj(f.Ctx(), lo, fname)
}

func (fos *FsObjSrv) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "Stat %v\n", f)
	o := f.Obj()
	st, r := o.Stat(f.Ctx())
	if r != nil {
		return r.Rerror()
	}
	rets.Stat = *st
	return nil
}

func (fos *FsObjSrv) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "Wstat %v %v\n", f, args)
	o := f.Obj()
	if args.Stat.Name != "" {
		// update Name atomically with rename

		if err := fos.fssrv.Sess(fos.sid).CheckFences(f.PathDir()); err != nil {
			log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), f.PathDir(), err)
			return err.Rerror()
		}

		dst := append(np.Copy(f.PathDir()), np.Split(args.Stat.Name)...)

		dws := fos.wt.WatchLookupL(f.PathDir())
		defer fos.wt.Release(dws)
		sws := fos.wt.WatchLookupL(f.Path())
		defer fos.wt.Release(sws)
		tws := fos.wt.WatchLookupL(dst)
		defer fos.wt.Release(tws)

		err := o.Parent().Rename(f.Ctx(), f.PathLast(), args.Stat.Name)
		if err != nil {
			return err.Rerror()
		}
		db.DLPrintf("9POBJ", "updateFid %v %v\n", f.PathLast(), dst)
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
	oldf, err := fos.lookup(args.OldFid)
	if err != nil {
		return err.Rerror()
	}
	newf, err := fos.lookup(args.NewFid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "Renameat %v %v %v\n", oldf, newf, args)
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
			log.Printf("%v %v Renameat CheckFences %v err %v\n", proc.GetName(), oldf.Ctx().Uname(), oldf.PathDir(), err)
			return err.Rerror()
		}
		if err := fos.fssrv.Sess(fos.sid).CheckFences(newf.PathDir()); err != nil {
			log.Printf("%v %v Renameat CheckFences %v err %v\n", proc.GetName(), newf.Ctx().Uname(), newf.PathDir(), err)
			return err.Rerror()
		}

		f1, f2 := lockOrder(oo, oldf, no, newf)
		d1ws := fos.wt.WatchLookupL(f1.Path())
		d2ws := fos.wt.WatchLookupL(f2.Path())
		defer fos.wt.Release(d1ws)
		defer fos.wt.Release(d2ws)

		src := append(np.Copy(oldf.Path()), args.OldName)
		dst := append(np.Copy(newf.Path()), args.NewName)
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

// Special code path for GetFile: in one RPC, GetFile() looks up the
// file, opens it, and reads it.  If an setfile executes between
// open() and read(), an getfile returns an error.
func (fos *FsObjSrv) GetFile(args np.Tgetfile, rets *np.Rgetfile) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "GetFile o %v args %v (%v)\n", f, args, len(args.Wnames))
	o := f.Obj()
	lo := o
	fname := append(f.Path(), args.Wnames...)
	if len(args.Wnames) > 0 {
		lo, err = fos.lookupObj(f.Ctx(), f, args.Wnames)
		if err != nil {
			return err.Rerror()
		}
	}
	if err := fos.fssrv.Sess(fos.sid).CheckFences(fname); err != nil {
		log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), fname, err)
		return err.Rerror()
	}
	fos.stats.Path(fname)
	_, r := lo.Open(f.Ctx(), args.Mode)
	if r != nil {
		return r.Rerror()
	}
	switch i := lo.(type) {
	case fs.Dir:
		return np.MkErr(np.TErrNotFile, fname).Rerror()
	case fs.File:
		rets.Data, r = i.Read(f.Ctx(), args.Offset, np.Tsize(lo.Size()), np.NoV)
		if r != nil {
			return r.Rerror()
		}
		return nil
	default:
		log.Fatalf("FATAL GetFile: obj type %T isn't Dir or File\n", o)

	}
	return nil
}

// Special code path for SetFile: in one RPC, SetFile() looks up the
// file, opens it, and writes it.
func (fos *FsObjSrv) SetFile(args np.Tsetfile, rets *np.Rwrite) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "SetFile o %v args %v (%v)\n", f, args, len(args.Wnames))
	// log.Printf("%v: SetFile o %v args %v (%v)\n", proc.GetName(), f, args, len(args.Wnames))
	lo := f.Obj()
	fname := append(f.Path(), args.Wnames...)
	if len(args.Wnames) > 0 {
		lo, err = fos.lookupObj(f.Ctx(), f, args.Wnames)
		if err != nil {
			return err.Rerror()
		}
	}

	if err := fos.fssrv.Sess(fos.sid).CheckFences(fname); err != nil {
		log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), fname, err)
		return err.Rerror()
	}

	fos.stats.Path(fname)
	_, err = lo.Open(f.Ctx(), args.Mode)
	if err != nil {
		return err.Rerror()
	}
	switch i := lo.(type) {
	case fs.Dir:
		return np.MkErr(np.TErrNotFile, f.Path()).Rerror()
	case fs.File:
		n, err := i.Write(f.Ctx(), args.Offset, args.Data, np.NoV)
		if err != nil {
			return err.Rerror()
		}
		rets.Count = n
		fos.rft.UpdateSeqno(fname)
		return nil
	default:
		log.Fatalf("FATAL SetFile: obj type %T isn't Dir or File\n", f)
	}
	return nil
}

// Special code path for PutFile: in one RPC, PutFile() looks up the
// file, creates it, and writes it.
func (fos *FsObjSrv) PutFile(args np.Tputfile, rets *np.Rwrite) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	db.DLPrintf("9POBJ", "PutFile o %v args %v (%v)\n", f, args, len(args.Wnames))
	// log.Printf("%v: PutFile o %v args %v (%v)\n", proc.GetName(), f, args, len(args.Wnames))
	lo := f.Obj()
	fname := append(f.Path(), args.Wnames...)
	names := args.Wnames[0 : len(args.Wnames)-1]
	dname := append(f.Path(), names...)
	if len(names) > 0 {
		lo, err = fos.lookupObj(f.Ctx(), f, names)
		if err != nil {
			return err.Rerror()
		}
	}

	if err := fos.fssrv.Sess(fos.sid).CheckFences(dname); err != nil {
		log.Printf("%v %v CheckFences %v err %v\n", proc.GetName(), f.Ctx().Uname(), dname, err)
		return err.Rerror()
	}

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

	switch i := lo.(type) {
	case fs.Dir:
		return np.MkErr(np.TErrNotFile, f.Path()).Rerror()
	case fs.File:
		n, err := i.Write(f.Ctx(), args.Offset, args.Data, np.NoV)
		if err != nil {
			return err.Rerror()
		}
		rets.Count = n
		fos.rft.UpdateSeqno(fname)
		return nil
	default:
		log.Fatalf("FATAL PutFile: obj type %T isn't Dir or File\n", f)
	}
	return nil
}

// XXX allow client to specify seqno and update it.
func (fos *FsObjSrv) MkFence(args np.Tmkfence, rets *np.Rmkfence) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	rets.Fence = fos.rft.MkFence(f.Path())
	// log.Printf("mkfence f %v -> %v\n", f.Path, rets.Fence)
	return nil
}

func (fos *FsObjSrv) RmFence(args np.Trmfence, rets *np.Ropen) *np.Rerror {
	_, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	// log.Printf("%v: rmfence %v %v\n", proc.GetName(), f.Path, args.Fence)
	err = fos.rft.RmFence(args.Fence)
	if err != nil {
		return err.Rerror()
	}
	return nil
}

func (fos *FsObjSrv) RegFence(args np.Tregfence, rets *np.Ropen) *np.Rerror {
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	// log.Printf("%v %v: RegFence %v %v\n", proc.GetName(), f.Ctx().Uname(), f.Path(), args)
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
	f, err := fos.lookup(args.Fid)
	if err != nil {
		return err.Rerror()
	}
	// log.Printf("%v: Unfence %v %v\n", proc.GetName(), f.Path(), args)
	err = fos.fssrv.Sess(fos.sid).Unfence(f.Path(), args.Fence.FenceId)
	if err != nil {
		return err.Rerror()
	}
	return nil
}
