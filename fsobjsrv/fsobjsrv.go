package fsobjsrv

import (
	"fmt"
	"log"
	"sync"

	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fs"
	"ulambda/fssrv"
	np "ulambda/ninep"
	"ulambda/protsrv"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/watch"
)

type FsObjSrv struct {
	mu     sync.Mutex // to protect closed
	closed bool

	fssrv *fssrv.FsServer
	wt    *watch.WatchTable
	st    *session.SessionTable
	stats *stats.Stats
}

type ProtServer struct{}

func MakeProtServer() protsrv.MakeProtServer {
	return &ProtServer{}
}

func (ps *ProtServer) MakeProtServer(s protsrv.FsServer) protsrv.Protsrv {
	fos := &FsObjSrv{}
	srv := s.(*fssrv.FsServer)
	fos.fssrv = srv
	fos.st = srv.SessionTable()
	fos.wt = srv.GetWatchTable()
	fos.stats = srv.GetStats()
	db.DLPrintf("NPOBJ", "MakeFsObjSrv -> %v", fos)
	return fos
}

func (fos *FsObjSrv) lookup(sess np.Tsession, fid np.Tfid) (*fid.Fid, *np.Rerror) {
	f, ok := fos.st.LookupFid(sess, fid)
	if !ok {
		return nil, np.ErrUnknownfid
	}
	return f, nil
}

func (fos *FsObjSrv) add(sess np.Tsession, fid np.Tfid, f *fid.Fid) {
	fos.st.AddFid(sess, fid, f)
}

func (fos *FsObjSrv) del(sess np.Tsession, fid np.Tfid) {
	o, ok := fos.st.DelFid(sess, fid)
	if ok && o.Perm().IsEphemeral() {
		fos.st.DelEphemeral(sess, o)
	}
}

func (fos *FsObjSrv) Closed() bool {
	defer fos.mu.Unlock()
	fos.mu.Lock()

	return fos.closed
}

func (fos *FsObjSrv) Version(sess np.Tsession, args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (fos *FsObjSrv) Auth(sess np.Tsession, args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.ErrUnknownMsg
}

func (fos *FsObjSrv) Attach(sess np.Tsession, args np.Tattach, rets *np.Rattach) *np.Rerror {
	path := np.Split(args.Aname)
	root, ctx := fos.fssrv.AttachTree(args.Uname, args.Aname)
	tree := root.(fs.FsObj)
	if args.Aname != "" {
		os, rest, err := root.Lookup(ctx, path)
		if len(rest) > 0 || err != nil {
			return &np.Rerror{err.Error()}
		}
		tree = os[len(os)-1]
	}
	fos.add(sess, args.Fid, fid.MakeFidPath(path, tree, 0, ctx))
	rets.Qid = tree.(fs.FsObj).Qid()
	return nil
}

// Delete ephemeral files created on this connection; caller
// is responsible for calling this serially, which should
// is not burden, because it is typically called once.
func (fos *FsObjSrv) Detach(sess np.Tsession) {
	ephemeral := fos.st.GetEphemeral(sess)
	db.DLPrintf("9POBJ", "Detach %v %v\n", sess, ephemeral)
	for o, f := range ephemeral {
		fos.removeObj(sess, f.Ctx(), o, f.Path())
	}
	fos.wt.DeleteConn(fos)
	fos.st.DeleteSession(sess)
	fos.fssrv.GetConnTable().Del(fos)

	fos.mu.Lock()
	defer fos.mu.Unlock()
	fos.closed = true
}

func makeQids(os []fs.FsObj) []np.Tqid {
	var qids []np.Tqid
	for _, o := range os {
		qids = append(qids, o.Qid())
	}
	return qids
}

func (fos *FsObjSrv) Walk(sess np.Tsession, args np.Twalk, rets *np.Rwalk) *np.Rerror {
	fos.stats.StatInfo().Nwalk.Inc()
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "Walk o %v args %v (%v)\n", f, args, len(args.Wnames))
	if len(args.Wnames) == 0 { // clone args.Fid?
		o := f.Obj()
		if o == nil {
			return np.ErrClunked
		}
		fos.add(sess, args.NewFid, fid.MakeFidPath(f.Path(), o, 0, f.Ctx()))
	} else {
		o := f.Obj()
		if o == nil {
			return np.ErrClunked
		}
		if !o.Perm().IsDir() {
			return np.ErrNotfound
		}
		d := o.(fs.Dir)
		os, rest, err := d.Lookup(f.Ctx(), args.Wnames)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		n := len(args.Wnames) - len(rest)
		p := append(f.Path(), args.Wnames[:n]...)
		lo := os[len(os)-1]
		fos.add(sess, args.NewFid, fid.MakeFidPath(p, lo, 0, f.Ctx()))
		rets.Qids = makeQids(os)
	}
	return nil
}

func (fos *FsObjSrv) Clunk(sess np.Tsession, args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	db.DLPrintf("9POBJ", "Clunk %v\n", args)
	fos.stats.StatInfo().Nclunk.Inc()
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	if f.Mode() != 0 { // has the fid been opened?
		o.Close(f.Ctx(), f.Mode())
	}
	fos.st.DelFid(sess, args.Fid)
	return nil
}

func (fos *FsObjSrv) Open(sess np.Tsession, args np.Topen, rets *np.Ropen) *np.Rerror {
	fos.stats.StatInfo().Nopen.Inc()
	db.DLPrintf("9POBJ", "Open %v\n", args)
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "f %v\n", f)
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	fos.stats.Path(f.Path())
	no, r := o.Open(f.Ctx(), args.Mode)
	if r != nil {
		return &np.Rerror{r.Error()}
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

func (fos *FsObjSrv) WatchV(sess np.Tsession, args np.Twatchv, rets *np.Ropen) *np.Rerror {
	fos.stats.StatInfo().Nwatchv.Inc()
	db.DLPrintf("9POBJ", "Watchv %v\n", args)

	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	p := f.Path()
	if len(args.Path) > 0 {
		p = append(p, args.Path...)
	}

	// get lock on watch entry for p, so that remove cannot remove
	// file before watch is set.
	ws := fos.wt.WatchLookupL(p)
	defer fos.wt.Release(ws)

	if o.Nlink() == 0 {
		return np.ErrNotfound
	}
	if !np.VEq(args.Version, o.Version()) {

		return &np.Rerror{fmt.Sprintf("Version mismatch %v %v %v", f.Path(), args.Version, o.Version())}
	}
	// time.Sleep(1000 * time.Nanosecond)

	ws.Watch(fos)

	return nil
}

func (fos *FsObjSrv) makeFid(sess np.Tsession, ctx fs.CtxI, dir []string, name string, o fs.FsObj, eph bool) *fid.Fid {
	p := np.Copy(dir)
	nf := fid.MakeFidPath(append(p, name), o, 0, ctx)
	if eph {
		fos.st.AddEphemeral(sess, o, nf)
	}
	return nf
}

// Create name in dir. If OWATCH is set and name already exits, wait
// until another thread deletes it, and retry.
func (fos *FsObjSrv) createObj(ctx fs.CtxI, d fs.Dir, dir []string, name string, perm np.Tperm, mode np.Tmode) (fs.FsObj, *np.Rerror) {
	p := append(dir, name)
	dws := fos.wt.WatchLookupL(dir)
	defer fos.wt.Release(dws)
	fws := fos.wt.WatchLookupL(p)
	defer fos.wt.Release(fws)
	for {
		o1, err := d.Create(ctx, name, perm, mode)
		db.DLPrintf("9POBJ", "Create %v %v %v ephemeral %v\n", name, o1, err, perm.IsEphemeral())
		if err == nil {
			fws.WakeupWatchL()
			dws.WakeupWatchL()
			return o1, nil
		} else {
			if mode&np.OWATCH == np.OWATCH && err.Error() == "Name exists" {
				fws.Unlock()
				err := dws.Watch(fos)
				fws.Lock() // not necessary if fail, but nicer with defer
				if err != nil {
					return nil, err
				}
				// try again; we will hold lock on watchers
			} else {
				return nil, &np.Rerror{err.Error()}
			}
		}
	}
}

func (fos *FsObjSrv) Create(sess np.Tsession, args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	fos.stats.StatInfo().Ncreate.Inc()
	db.DLPrintf("9POBJ", "Create %v\n", args)
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "f %v\n", f)
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}

	names := []string{args.Name}
	if !o.Perm().IsDir() {
		return &np.Rerror{fmt.Sprintf("Not a directory")}
	}

	d := o.(fs.Dir)
	o1, r := fos.createObj(f.Ctx(), d, f.Path(), names[0], args.Perm, args.Mode)
	if r != nil {
		return r
	}
	nf := fos.makeFid(sess, f.Ctx(), f.Path(), names[0], o1, args.Perm.IsEphemeral())
	fos.add(sess, args.Fid, nf)
	rets.Qid = o1.Qid()
	return nil
}

func (fos *FsObjSrv) Flush(sess np.Tsession, args np.Tflush, rets *np.Rflush) *np.Rerror {
	fos.stats.StatInfo().Nflush.Inc()
	return nil
}

func (fos *FsObjSrv) Read(sess np.Tsession, args np.Tread, rets *np.Rread) *np.Rerror {
	fos.stats.StatInfo().Nread.Inc()
	db.DLPrintf("9POBJ", "Read %v\n", args)
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "ReadFid %v %v\n", args, f)
	return f.Read(args.Offset, args.Count, np.NoV, rets)
}

func (fos *FsObjSrv) Write(sess np.Tsession, args np.Twrite, rets *np.Rwrite) *np.Rerror {
	fos.stats.StatInfo().Nwrite.Inc()
	db.DLPrintf("9POBJ", "Write %v\n", args)
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	var r *np.Rerror
	rets.Count, r = f.Write(args.Offset, args.Data, np.NoV)
	return r
}

// XXX make .exit a reserved name
func isExit(path []string) bool {
	return len(path) == 1 && path[0] == ".exit"
}

func (fos *FsObjSrv) removeObj(sess np.Tsession, ctx fs.CtxI, o fs.FsObj, path []string) *np.Rerror {

	// log.Printf("removeObj %v\n", path)

	// lock watch entry to make WatchV and Remove interact
	// correctly
	dws := fos.wt.WatchLookupL(np.Dir(path))
	fws := fos.wt.WatchLookupL(path)
	defer fos.wt.Release(dws)
	defer fos.wt.Release(fws)

	fos.stats.Path(path)

	r := o.Parent().Remove(ctx, path[len(path)-1])
	if r != nil {
		log.Printf("remove err %v f %v\n", r, path)
		return &np.Rerror{r.Error()}
	}
	r = o.Unlink(ctx)
	if r != nil {
		log.Printf("Unlink err %v f %v\n", r, path)
		return &np.Rerror{r.Error()}
	}

	fws.WakeupWatchL()
	dws.WakeupWatchL()

	if o.Perm().IsEphemeral() {
		fos.st.DelEphemeral(sess, o)
	}
	return nil
}

func (fos *FsObjSrv) Remove(sess np.Tsession, args np.Tremove, rets *np.Rremove) *np.Rerror {
	fos.stats.StatInfo().Nremove.Inc()
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	if isExit(f.Path()) {
		db.DLPrintf("9POBJ", "Done\n")
		fos.fssrv.Done()
		return nil
	}
	return fos.removeObj(sess, f.Ctx(), o, f.Path())
}

func (fos *FsObjSrv) lookupObj(ctx fs.CtxI, o fs.FsObj, names []string) (fs.FsObj, *np.Rerror) {
	if !o.Perm().IsDir() {
		return nil, np.ErrNotfound
	}
	d := o.(fs.Dir)
	os, rest, err := d.Lookup(ctx, names)
	if err != nil || len(rest) != 0 {
		return nil, &np.Rerror{fmt.Errorf("dir not found %v", names).Error()}
	}
	return os[len(os)-1], nil
}

// RemoveFile is Remove() but args.Wnames may contain a symlink that
// hasn't been walked. If so, RemoveFile() will not succeed looking up
// args.Wnames, and caller should first walk the pathname.
func (fos *FsObjSrv) RemoveFile(sess np.Tsession, args np.Tremovefile, rets *np.Rremove) *np.Rerror {
	var err *np.Rerror
	fos.stats.StatInfo().Nremove.Inc()
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}

	fname := append(f.Path(), args.Wnames[0:len(args.Wnames)]...)
	if isExit(fname) {
		db.DLPrintf("9POBJ", "Done\n")
		fos.fssrv.Done()
		return nil
	}

	lo := o
	if len(args.Wnames) > 0 {
		lo, err = fos.lookupObj(f.Ctx(), o, args.Wnames)
		if err != nil {
			log.Printf("lookup err %v f %v\n", err, fname)
			return err
		}
	}
	return fos.removeObj(sess, f.Ctx(), lo, fname)
}

func (fos *FsObjSrv) Stat(sess np.Tsession, args np.Tstat, rets *np.Rstat) *np.Rerror {
	fos.stats.StatInfo().Nstat.Inc()
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "Stat %v\n", f)
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	st, r := o.Stat(f.Ctx())
	if r != nil {
		return &np.Rerror{r.Error()}
	}
	rets.Stat = *st
	return nil
}

func (fos *FsObjSrv) Wstat(sess np.Tsession, args np.Twstat, rets *np.Rwstat) *np.Rerror {
	fos.stats.StatInfo().Nwstat.Inc()
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "Wstat %v %v\n", f, args)
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	if args.Stat.Name != "" {
		// update Name atomically with rename

		dst := append(np.Copy(f.PathDir()), np.Split(args.Stat.Name)...)

		dws := fos.wt.WatchLookupL(f.PathDir())
		defer fos.wt.Release(dws)
		sws := fos.wt.WatchLookupL(f.Path())
		defer fos.wt.Release(sws)
		tws := fos.wt.WatchLookupL(dst)
		defer fos.wt.Release(tws)

		err := o.Parent().Rename(f.Ctx(), f.PathLast(), args.Stat.Name)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		db.DLPrintf("9POBJ", "updateFid %v %v\n", f.PathLast(), dst)
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
		log.Fatalf("lockOrder")
		return oldf, newf
	} else {
		return newf, oldf
	}
}

func (fos *FsObjSrv) Renameat(sess np.Tsession, args np.Trenameat, rets *np.Rrenameat) *np.Rerror {
	fos.stats.StatInfo().Nrenameat.Inc()
	oldf, err := fos.lookup(sess, args.OldFid)
	if err != nil {
		return err
	}
	newf, err := fos.lookup(sess, args.NewFid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "Renameat %v %v %v\n", oldf, newf, args)
	oo := oldf.Obj()
	if oo == nil {
		return np.ErrClunked
	}
	no := newf.Obj()
	if oo == nil {
		return np.ErrClunked
	}
	switch d1 := oo.(type) {
	case fs.Dir:
		d2, ok := no.(fs.Dir)
		if !ok {
			return np.ErrNotDir
		}
		if oo.Inum() == no.Inum() {
			return np.ErrInval
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

			return &np.Rerror{err.Error()}
		}
		dstws.WakeupWatchL() // trigger create watch
		srcws.WakeupWatchL() // trigger remove watch
		d1ws.WakeupWatchL()  // trigger one dir watch
		d2ws.WakeupWatchL()  // trigger the other dir watch
	default:
		return np.ErrNotDir
	}
	return nil
}

// Special code path for GetFile: in one RPC, GetFile() looks up the file,
// opens it, and reads it.
func (fos *FsObjSrv) GetFile(sess np.Tsession, args np.Tgetfile, rets *np.Rgetfile) *np.Rerror {
	fos.stats.StatInfo().Nget.Inc()
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "GetFile o %v args %v (%v)\n", f, args, len(args.Wnames))
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	lo := o
	if len(args.Wnames) > 0 {
		lo, err = fos.lookupObj(f.Ctx(), o, args.Wnames)
		if err != nil {
			return err
		}
	}
	fos.stats.Path(f.Path())
	_, r := lo.Open(f.Ctx(), args.Mode)
	if r != nil {
		return &np.Rerror{r.Error()}
	}
	v := lo.Version()
	switch i := lo.(type) {
	case fs.Dir:
		return np.ErrNotFile
	case fs.File:
		rets.Data, r = i.Read(f.Ctx(), args.Offset, np.Tsize(lo.Size()), v)
		rets.Version = v
		if r != nil {
			return &np.Rerror{r.Error()}
		}
		return nil
	default:
		log.Fatalf("GetFile: obj type %T isn't Dir or File\n", o)

	}
	return nil
}

// Special code path for SetFile: in one RPC, SetFile() looks up the
// file, opens/creates it, and writes it.
func (fos *FsObjSrv) SetFile(sess np.Tsession, args np.Tsetfile, rets *np.Rwrite) *np.Rerror {
	var r error
	var err *np.Rerror
	fos.stats.StatInfo().Nset.Inc()
	f, err := fos.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "SetFile o %v args %v (%v)\n", f, args, len(args.Wnames))
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	names := args.Wnames
	lo := o
	if args.Perm != 0 { // create?
		names = names[0 : len(args.Wnames)-1]
	}
	dname := append(f.Path(), names[0:len(args.Wnames)-1]...)
	if len(names) > 0 {
		lo, err = fos.lookupObj(f.Ctx(), o, names)
		if err != nil {
			return err
		}
	}
	if args.Perm != 0 { // create?
		if !lo.Perm().IsDir() {
			return &np.Rerror{fmt.Errorf("dir not found %v", args.Wnames).Error()}
		}
		name := args.Wnames[len(args.Wnames)-1]
		lo, err = fos.createObj(f.Ctx(), lo.(fs.Dir), dname, name, args.Perm, args.Mode)
		if err != nil {
			return err
		}
		fos.makeFid(sess, f.Ctx(), dname, name, lo, args.Perm.IsEphemeral())
	} else {
		fos.stats.Path(f.Path())
		_, r = lo.Open(f.Ctx(), args.Mode)
		if r != nil {
			return &np.Rerror{r.Error()}
		}
	}
	// time.Sleep(1000 * time.Nanosecond)
	switch i := lo.(type) {
	case fs.Dir:
		return np.ErrNotFile
	case fs.File:
		rets.Count, r = i.Write(f.Ctx(), args.Offset, args.Data, args.Version)
		if r != nil {
			return &np.Rerror{r.Error()}
		}
		return nil
	default:
		log.Fatalf("SetFile: obj type %T isn't Dir or File\n", o)

	}
	return nil
}

func (fos *FsObjSrv) Register(sess np.Tsession, args np.Tregister, rets *np.Ropen) *np.Rerror {
	return nil
}
