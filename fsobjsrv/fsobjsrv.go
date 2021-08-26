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

type NpConn struct {
	mu     sync.Mutex
	closed bool
	fssrv  *fssrv.FsServer
	wt     *watch.WatchTable
	st     *session.SessionTable
	stats  *stats.Stats
}

type ProtServer struct{}

func MakeProtServer() protsrv.MakeProtServer {
	return &ProtServer{}
}

func (ncm *ProtServer) MakeProtServer(s protsrv.FsServer) protsrv.Protsrv {
	npc := &NpConn{}
	srv := s.(*fssrv.FsServer)
	npc.fssrv = srv
	npc.st = srv.SessionTable()
	npc.wt = srv.GetWatchTable()
	npc.stats = srv.GetStats()
	db.DLPrintf("NPOBJ", "MakeNpConn -> %v", npc)
	return npc
}

func (npc *NpConn) lookup(sess np.Tsession, fid np.Tfid) (*fid.Fid, *np.Rerror) {
	f, ok := npc.st.LookupFid(sess, fid)
	if !ok {
		return nil, np.ErrUnknownfid
	}
	return f, nil
}

func (npc *NpConn) add(sess np.Tsession, fid np.Tfid, f *fid.Fid) {
	npc.st.AddFid(sess, fid, f)
}

func (npc *NpConn) del(sess np.Tsession, fid np.Tfid) {
	o := npc.st.DelFid(sess, fid)
	if o.Perm().IsEphemeral() {
		npc.st.DelEphemeral(sess, o)
	}
}

func (npc *NpConn) Closed() bool {
	defer npc.mu.Unlock()
	npc.mu.Lock()

	return npc.closed
}

func (npc *NpConn) Version(sess np.Tsession, args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(sess np.Tsession, args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.ErrUnknownMsg
}

func (npc *NpConn) Attach(sess np.Tsession, args np.Tattach, rets *np.Rattach) *np.Rerror {
	root, ctx := npc.fssrv.RootAttach(args.Uname)
	npc.add(sess, args.Fid, fid.MakeFid(root, ctx))
	rets.Qid = root.Qid()
	return nil
}

// Delete ephemeral files created on this connection; caller
// is responsible for calling this serially, which should
// is not burden, because it is typically called once.
func (npc *NpConn) Detach(sess np.Tsession) {

	ephemeral := npc.st.GetEphemeral(sess)
	db.DLPrintf("9POBJ", "Detach %v %v\n", sess, ephemeral)
	for o, f := range ephemeral {
		o.Remove(f.Ctx(), f.PathLast())
		npc.wt.WakeupWatch(f.Path(), f.PathDir())
	}
	npc.wt.DeleteConn(npc)
	npc.fssrv.GetConnTable().Del(npc)
	npc.mu.Lock()
	npc.closed = true
	npc.mu.Unlock()
}

func makeQids(os []fs.FsObj) []np.Tqid {
	var qids []np.Tqid
	for _, o := range os {
		qids = append(qids, o.Qid())
	}
	return qids
}

func (npc *NpConn) Walk(sess np.Tsession, args np.Twalk, rets *np.Rwalk) *np.Rerror {
	npc.stats.StatInfo().Nwalk.Inc()
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "Walk o %v args %v (%v)\n", f, args, len(args.Wnames))
	if len(args.Wnames) == 0 { // clone args.Fid?
		o := f.Obj()
		if o == nil {
			return np.ErrClunked
		}
		npc.add(sess, args.NewFid, fid.MakeFid(o, f.Ctx()))
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
		npc.add(sess, args.NewFid, fid.MakeFidPath(p, lo, f.Ctx()))
		rets.Qids = makeQids(os)
	}
	return nil
}

func (npc *NpConn) Clunk(sess np.Tsession, args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	db.DLPrintf("9POBJ", "Clunk %v\n", args)
	npc.stats.StatInfo().Nclunk.Inc()
	_, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	npc.st.DelFid(sess, args.Fid)
	return nil
}

func (npc *NpConn) Open(sess np.Tsession, args np.Topen, rets *np.Ropen) *np.Rerror {
	npc.stats.StatInfo().Nopen.Inc()
	db.DLPrintf("9POBJ", "Open %v\n", args)
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "f %v\n", f)
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	npc.stats.Path(f.Path())
	r := o.Open(f.Ctx(), args.Mode)
	if err != nil {
		return &np.Rerror{r.Error()}
	}
	rets.Qid = o.Qid()
	return nil
}

func (npc *NpConn) WatchV(sess np.Tsession, args np.Twatchv, rets *np.Ropen) *np.Rerror {
	npc.stats.StatInfo().Nwatchv.Inc()
	db.DLPrintf("9POBJ", "Watchv %v\n", args)
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	if args.Version != np.NoV && args.Version != o.Version() {
		s := fmt.Sprintf("Version mismatch %v %v %v", f.Path(), args.Version, o.Version())
		return &np.Rerror{s}
	}
	p := f.Path()
	if len(args.Path) > 0 {
		p = append(p, args.Path...)
	}
	ws := npc.wt.WatchLookupL(p)
	ws.Watch(npc)
	return nil
}

func (npc *NpConn) makeFid(sess np.Tsession, ctx fs.CtxI, dir []string, name string, o fs.FsObj, eph bool) *fid.Fid {
	p := np.Copy(dir)
	nf := fid.MakeFidPath(append(p, name), o, ctx)
	if eph {
		npc.st.AddEphemeral(sess, o, nf)
	}
	npc.wt.WakeupWatch(nf.Path(), dir)
	return nf
}

// Create name in dir. If OWATCH is set and name already exits, wait
// until another thread deletes it, and retry.
func (npc *NpConn) createObj(ctx fs.CtxI, d fs.Dir, dir []string, name string, perm np.Tperm, mode np.Tmode) (fs.FsObj, *np.Rerror) {
	for {
		p := append(dir, name)
		var ws *watch.Watchers
		if mode&np.OWATCH == np.OWATCH {
			ws = npc.wt.WatchLookupL(p)
		}
		o1, err := d.Create(ctx, name, perm, mode)
		db.DLPrintf("9POBJ", "Create %v %v %v ephemeral %v\n", name, o1, err, perm.IsEphemeral())
		if err == nil {
			if ws != nil {
				npc.wt.Release(ws, p)
			}
			return o1, nil
		} else {
			if ws != nil && err.Error() == "Name exists" {
				err := ws.Watch(npc)
				if err != nil {
					return nil, err
				}
				// try again
			} else {
				if ws != nil {
					npc.wt.Release(ws, p)
				}
				return nil, &np.Rerror{err.Error()}
			}
		}
	}
}

func (npc *NpConn) Create(sess np.Tsession, args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	npc.stats.StatInfo().Ncreate.Inc()
	db.DLPrintf("9POBJ", "Create %v\n", args)
	f, err := npc.lookup(sess, args.Fid)
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
	o1, r := npc.createObj(f.Ctx(), d, f.Path(), names[0], args.Perm, args.Mode)
	if r != nil {
		return r
	}
	nf := npc.makeFid(sess, f.Ctx(), f.Path(), names[0], o1, args.Perm.IsEphemeral())
	npc.add(sess, args.Fid, nf)
	rets.Qid = o1.Qid()
	return nil
}

func (npc *NpConn) Flush(sess np.Tsession, args np.Tflush, rets *np.Rflush) *np.Rerror {
	npc.stats.StatInfo().Nflush.Inc()
	return nil
}

func (npc *NpConn) Read(sess np.Tsession, args np.Tread, rets *np.Rread) *np.Rerror {
	npc.stats.StatInfo().Nread.Inc()
	db.DLPrintf("9POBJ", "Read %v\n", args)
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "ReadFid %v %v\n", args, f)
	return f.Read(args.Offset, args.Count, np.NoV, rets)
}

func (npc *NpConn) Write(sess np.Tsession, args np.Twrite, rets *np.Rwrite) *np.Rerror {
	npc.stats.StatInfo().Nwrite.Inc()
	db.DLPrintf("9POBJ", "Write %v\n", args)
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	var r *np.Rerror
	rets.Count, r = f.Write(args.Offset, args.Data, np.NoV)
	return r
}

func (npc *NpConn) Remove(sess np.Tsession, args np.Tremove, rets *np.Rremove) *np.Rerror {
	npc.stats.StatInfo().Nremove.Inc()
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	if len(f.Path()) == 0 { // exit?
		db.DLPrintf("9POBJ", "Done\n")
		npc.fssrv.Done()
		return nil
	}
	db.DLPrintf("9POBJ", "Remove f %v\n", f)
	r := o.Remove(f.Ctx(), f.PathLast())
	if r != nil {
		return &np.Rerror{r.Error()}
	}
	db.DLPrintf("9POBJ", "Remove f WakeupWatch %v\n", f)
	npc.wt.WakeupWatch(f.Path(), f.PathDir())

	// delete from ephemeral table, if ephemeral
	npc.del(sess, args.Fid)
	return nil
}

func (npc *NpConn) lookupObj(ctx fs.CtxI, o fs.FsObj, names []string) (fs.FsObj, *np.Rerror) {
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

func (npc *NpConn) RemoveFile(sess np.Tsession, args np.Tremovefile, rets *np.Rremove) *np.Rerror {
	var err *np.Rerror
	npc.stats.StatInfo().Nremove.Inc()
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	lo := o
	if len(f.Path()) == 0 && len(args.Wnames) == 1 && args.Wnames[0] == "." { // exit?
		db.DLPrintf("9POBJ", "Done\n")
		npc.fssrv.Done()
		return nil
	}
	if len(args.Wnames) > 0 {
		lo, err = npc.lookupObj(f.Ctx(), o, args.Wnames)
		if err != nil {
			return err
		}
	}
	npc.stats.Path(f.Path())
	fname := append(f.Path(), args.Wnames[0:len(args.Wnames)]...)
	dname := append(f.Path(), args.Wnames[0:len(args.Wnames)-1]...)
	r := lo.Remove(f.Ctx(), fname[len(fname)-1])
	if r != nil {
		return &np.Rerror{r.Error()}
	}

	npc.wt.WakeupWatch(fname, dname)

	if lo.Perm().IsEphemeral() {
		npc.st.DelEphemeral(sess, lo)
	}
	return nil
}

func (npc *NpConn) Stat(sess np.Tsession, args np.Tstat, rets *np.Rstat) *np.Rerror {
	npc.stats.StatInfo().Nstat.Inc()
	f, err := npc.lookup(sess, args.Fid)
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

func (npc *NpConn) Wstat(sess np.Tsession, args np.Twstat, rets *np.Rwstat) *np.Rerror {
	npc.stats.StatInfo().Nwstat.Inc()
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "Wstat %v %v\n", f, args)
	o := f.Obj()
	if o == nil {
		return np.ErrClunked
	}
	if args.Stat.Name != "" {
		err := o.Rename(f.Ctx(), f.PathLast(), args.Stat.Name)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		dst := append(f.PathDir(), np.Split(args.Stat.Name)...)
		db.DLPrintf("9POBJ", "dst %v %v %v\n", dst, f.PathLast(), args.Stat.Name)
		f.SetPath(dst)
		npc.wt.WakeupWatch(dst, nil)
	}
	// XXX ignore other Wstat for now
	return nil
}

func (npc *NpConn) Renameat(sess np.Tsession, args np.Trenameat, rets *np.Rrenameat) *np.Rerror {
	npc.stats.StatInfo().Nrenameat.Inc()
	oldf, err := npc.lookup(sess, args.OldFid)
	if err != nil {
		return err
	}
	newf, err := npc.lookup(sess, args.NewFid)
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
		err := d1.Renameat(oldf.Ctx(), args.OldName, d2, args.NewName)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		dst := np.Copy(newf.Path())
		dst = append(dst, args.NewName)
		npc.wt.WakeupWatch(dst, nil)
	default:
		return np.ErrNotDir
	}
	return nil
}

// Special code path for GetFile: in one RPC, GetFile() looks up the file,
// opens it, and reads it.
func (npc *NpConn) GetFile(sess np.Tsession, args np.Tgetfile, rets *np.Rgetfile) *np.Rerror {
	npc.stats.StatInfo().Nget.Inc()
	f, err := npc.lookup(sess, args.Fid)
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
		lo, err = npc.lookupObj(f.Ctx(), o, args.Wnames)
		if err != nil {
			return err
		}
	}
	npc.stats.Path(f.Path())
	r := lo.Open(f.Ctx(), args.Mode)
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
func (npc *NpConn) SetFile(sess np.Tsession, args np.Tsetfile, rets *np.Rwrite) *np.Rerror {
	var r error
	var err *np.Rerror
	npc.stats.StatInfo().Nset.Inc()
	f, err := npc.lookup(sess, args.Fid)
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
		lo, err = npc.lookupObj(f.Ctx(), o, names)
		if err != nil {
			return err
		}
	}
	if args.Perm != 0 { // create?
		if !lo.Perm().IsDir() {
			return &np.Rerror{fmt.Errorf("dir not found %v", args.Wnames).Error()}
		}
		name := args.Wnames[len(args.Wnames)-1]
		lo, err = npc.createObj(f.Ctx(), lo.(fs.Dir), dname, name, args.Perm, args.Mode)
		if err != nil {
			return err
		}
		npc.makeFid(sess, f.Ctx(), dname, name, lo, args.Perm.IsEphemeral())
	} else {
		npc.stats.Path(f.Path())
		r = lo.Open(f.Ctx(), args.Mode)
		if r != nil {
			return &np.Rerror{r.Error()}
		}
	}
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
