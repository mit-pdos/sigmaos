package npobjsrv

import (
	"fmt"
	"log"
	"net"
	"sync"

	db "ulambda/debug"
	"ulambda/fssrv"
	np "ulambda/ninep"
	"ulambda/stats"
)

type NpConn struct {
	mu     sync.Mutex
	closed bool
	conn   net.Conn
	osrv   NpObjSrv
	wt     *WatchTable
	ct     *ConnTable
	st     *SessionTable
	stats  *stats.Stats
}

func MakeNpConn(osrv NpObjSrv, srv *fssrv.FsServer, conn net.Conn) *NpConn {
	npc := &NpConn{}
	npc.conn = conn
	npc.osrv = osrv
	npc.wt = osrv.WatchTable()
	npc.ct = osrv.ConnTable()
	npc.st = osrv.SessionTable()
	npc.stats = srv.GetStats()
	if npc.ct != nil {
		npc.ct.Add(npc)
	}
	db.DLPrintf("NPOBJ", "MakeNpConn %v -> %v", conn, npc)
	return npc
}

func (npc *NpConn) Addr() string {
	return npc.conn.LocalAddr().String()
}

func (npc *NpConn) lookup(sess np.Tsession, fid np.Tfid) (*Fid, *np.Rerror) {
	f, ok := npc.st.lookupFid(sess, fid)
	if !ok {
		return nil, np.ErrUnknownfid
	}
	return f, nil
}

func (npc *NpConn) add(sess np.Tsession, fid np.Tfid, f *Fid) {
	npc.st.addFid(sess, fid, f)
}

func (npc *NpConn) del(sess np.Tsession, fid np.Tfid) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	o := npc.st.delFid(sess, fid)
	if o.Perm().IsEphemeral() {
		npc.st.delEphemeral(sess, o)
	}
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
	root, ctx := npc.osrv.RootAttach(args.Uname)
	npc.add(sess, args.Fid, &Fid{sync.Mutex{}, []string{}, root, root.Version(), ctx})
	rets.Qid = root.Qid()
	return nil
}

// Delete ephemeral files created on this connection
func (npc *NpConn) Detach(sess np.Tsession) {

	npc.mu.Lock()
	ephemeral := npc.st.getEphemeral(sess)
	db.DLPrintf("9POBJ", "Detach %v %v\n", sess, ephemeral)
	for o, f := range ephemeral {
		o.Remove(f.ctx, f.path[len(f.path)-1])
		if npc.wt != nil {
			// Wake up watches on parent dir as well
			npc.wt.WakeupWatch(f.path, f.path[:len(f.path)-1])
		}
	}
	npc.mu.Unlock()

	if npc.wt != nil {
		npc.wt.DeleteConn(npc)
	}
	if npc.ct != nil {
		npc.ct.Del(npc)
	}
	npc.mu.Lock()
	npc.closed = true
	npc.mu.Unlock()
}

func makeQids(os []NpObj) []np.Tqid {
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
			return &np.Rerror{"Closed by server"}
		}
		npc.add(sess, args.NewFid, &Fid{sync.Mutex{}, f.path, o, o.Version(), f.ctx})
	} else {
		o := f.Obj()
		if o == nil {
			return &np.Rerror{"Closed by server"}
		}
		if !o.Perm().IsDir() {
			return np.ErrNotfound
		}
		d := o.(NpObjDir)
		os, rest, err := d.Lookup(f.ctx, args.Wnames)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		n := len(args.Wnames) - len(rest)
		p := append(f.path, args.Wnames[:n]...)
		lo := os[len(os)-1]
		npc.add(sess, args.NewFid, &Fid{sync.Mutex{}, p, lo, lo.Version(), f.ctx})
		rets.Qids = makeQids(os)
	}
	return nil
}

// XXX call close? keep refcnt per obj?
func (npc *NpConn) Clunk(sess np.Tsession, args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	db.DLPrintf("9POBJ", "Clunk %v\n", args)
	npc.stats.StatInfo().Nclunk.Inc()
	_, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	npc.st.delFid(sess, args.Fid)
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
		return &np.Rerror{"Closed by server"}
	}
	npc.stats.Path(f.path)
	r := o.Open(f.ctx, args.Mode)
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
		return &np.Rerror{"Closed by server"}
	}
	if args.Version != np.NoV && args.Version != o.Version() {
		s := fmt.Sprintf("Version mismatch %v %v %v", f.path, args.Version, o.Version())
		return &np.Rerror{s}
	}
	p := f.path
	if len(args.Path) > 0 {
		p = append(p, args.Path...)
	}
	ws := npc.wt.WatchLookup(p)
	ws.Watch(npc)
	return nil
}

func (npc *NpConn) makeFid(sess np.Tsession, ctx CtxI, dir []string, name string, o NpObj, eph bool) *Fid {
	p := np.Copy(dir)
	nf := &Fid{sync.Mutex{}, append(p, name), o, o.Version(), ctx}
	if eph {
		npc.mu.Lock()
		npc.st.addEphemeral(sess, o, nf)
		npc.mu.Unlock()
	}
	if npc.wt != nil {
		npc.wt.WakeupWatch(nf.path, dir)
	}
	return nf
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
		return &np.Rerror{"Closed by server"}
	}

	names := []string{args.Name}
	if !o.Perm().IsDir() {
		return &np.Rerror{fmt.Sprintf("Not a directory")}
	}
	for {
		d := o.(NpObjDir)
		var ws *Watchers
		if npc.wt != nil {
			ws = npc.wt.WatchLookup(append(f.path, names[0]))
		}
		o1, err := d.Create(f.ctx, names[0], args.Perm, args.Mode)
		db.DLPrintf("9POBJ", "Create %v %v %v\n", names[0], o1, err)
		if err == nil {
			if ws != nil {
				ws.mu.Unlock()
			}
			nf := npc.makeFid(sess, f.ctx, f.path, names[0], o1, args.Perm.IsEphemeral())
			npc.add(sess, args.Fid, nf)
			rets.Qid = o1.Qid()
			break
		} else {
			if npc.wt != nil && err.Error() == "Name exists" && args.Mode&np.OWATCH == np.OWATCH {
				err := ws.Watch(npc)
				if err != nil {
					return err
				}
			} else {
				if ws != nil {
					ws.mu.Unlock()
				}
				return &np.Rerror{err.Error()}
			}
		}
	}
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
	var r error
	rets.Count, r = f.Write(args.Offset, args.Data, np.NoV)
	if r != nil {
		return &np.Rerror{r.Error()}
	}
	return nil
}

func (npc *NpConn) Remove(sess np.Tsession, args np.Tremove, rets *np.Rremove) *np.Rerror {
	npc.stats.StatInfo().Nremove.Inc()
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return &np.Rerror{"Closed by server"}
	}
	if len(f.path) == 0 { // exit?
		db.DLPrintf("9POBJ", "Done\n")
		npc.osrv.Done()
		return nil
	}
	db.DLPrintf("9POBJ", "Remove f %v\n", f)
	r := o.Remove(f.ctx, f.path[len(f.path)-1])
	if r != nil {
		return &np.Rerror{r.Error()}
	}
	if npc.wt != nil {
		db.DLPrintf("9POBJ", "Remove f WakeupWatch %v\n", f)
		// Wake up watches on parent dir as well
		npc.wt.WakeupWatch(f.path, f.path[:len(f.path)-1])
	}
	// delete from ephemeral table, if ephemeral
	npc.del(sess, args.Fid)
	return nil
}

func (npc *NpConn) RemoveFile(sess np.Tsession, args np.Tremovefile, rets *np.Rremove) *np.Rerror {
	npc.stats.StatInfo().Nremove.Inc()
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	o := f.Obj()
	if o == nil {
		return &np.Rerror{"Closed by server"}
	}
	lo := o
	if len(f.path) == 0 && len(args.Wnames) == 1 && args.Wnames[0] == "." { // exit?
		db.DLPrintf("9POBJ", "Done\n")
		npc.osrv.Done()
		return nil
	}
	if len(args.Wnames) > 0 {
		if !o.Perm().IsDir() {
			return np.ErrNotfound
		}
		d := o.(NpObjDir)
		os, rest, err := d.Lookup(f.ctx, args.Wnames)
		if err != nil || len(rest) != 0 {
			return &np.Rerror{fmt.Errorf("dir not found %v", args.Wnames).Error()}
		}
		lo = os[len(os)-1]
	}
	npc.stats.Path(f.path)
	fname := append(f.path, args.Wnames[0:len(args.Wnames)]...)
	dname := append(f.path, args.Wnames[0:len(args.Wnames)-1]...)
	r := lo.Remove(f.ctx, fname[len(fname)-1])
	if r != nil {
		return &np.Rerror{r.Error()}
	}
	if npc.wt != nil {
		// Wake up watches on parent dir as well
		npc.wt.WakeupWatch(fname, dname)
	}
	// XXX delete from ephemeral table, if ephemeral
	//	npc.del(sess, args.Fid) // XXX doing this here causes "unkown Fid" errors in fslib tests
	if lo.Perm().IsEphemeral() {
		npc.st.delEphemeral(sess, lo)
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
		return &np.Rerror{"Closed by server"}
	}
	st, r := o.Stat(f.ctx)
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
		return &np.Rerror{"Closed by server"}
	}
	if args.Stat.Name != "" {
		// XXX if dst exists run watch?
		err := o.Rename(f.ctx, f.path[len(f.path)-1], args.Stat.Name)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		dst := append(f.path[:len(f.path)-1], np.Split(args.Stat.Name)...)
		db.DLPrintf("9POBJ", "dst %v %v %v\n", dst, f.path[len(f.path)-1], args.Stat.Name)
		f.path = dst
		if npc.wt != nil {
			npc.wt.WakeupWatch(dst, nil)
		}
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
		return &np.Rerror{"Closed by server"}
	}
	no := newf.Obj()
	if oo == nil {
		return &np.Rerror{"Closed by server"}
	}
	switch d1 := oo.(type) {
	case NpObjDir:
		d2, ok := no.(NpObjDir)
		if !ok {
			return np.ErrNotDir
		}
		err := d1.Renameat(oldf.ctx, args.OldName, d2, args.NewName)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		if npc.wt != nil {
			dst := np.Copy(newf.path)
			dst = append(dst, args.NewName)
			npc.wt.WakeupWatch(dst, nil)
		}
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
		return &np.Rerror{"Closed by server"}
	}
	lo := o
	if len(args.Wnames) > 0 {
		if !o.Perm().IsDir() {
			return np.ErrNotfound
		}
		d := o.(NpObjDir)
		os, rest, err := d.Lookup(f.ctx, args.Wnames)
		if err != nil || len(rest) != 0 {
			return &np.Rerror{fmt.Errorf("dir not found %v", args.Wnames).Error()}
		}
		lo = os[len(os)-1]
	}
	npc.stats.Path(f.path)
	r := lo.Open(f.ctx, args.Mode)
	if r != nil {
		return &np.Rerror{r.Error()}
	}
	v := lo.Version()
	switch i := lo.(type) {
	case NpObjDir:
		return np.ErrNotFile
	case NpObjFile:
		rets.Data, r = i.Read(f.ctx, args.Offset, np.Tsize(lo.Size()), v)
		rets.Version = v
		if r != nil {
			return &np.Rerror{r.Error()}
		}
		return nil
	default:
		log.Fatalf("GetFile: obj type %T isn't NpObjDir or NpObjFile\n", o)

	}
	return nil
}

// Special code path for SetFile: in one RPC, SetFile() looks up the
// file, opens/creates it, and writes it.
func (npc *NpConn) SetFile(sess np.Tsession, args np.Tsetfile, rets *np.Rwrite) *np.Rerror {
	var r error
	npc.stats.StatInfo().Nset.Inc()
	f, err := npc.lookup(sess, args.Fid)
	if err != nil {
		return err
	}
	db.DLPrintf("9POBJ", "SetFile o %v args %v (%v)\n", f, args, len(args.Wnames))
	o := f.Obj()
	if o == nil {
		return &np.Rerror{"Closed by server"}
	}
	names := args.Wnames
	lo := o
	if args.Perm != 0 { // create?
		names = names[0 : len(args.Wnames)-1]
	}
	dname := append(f.path, names[0:len(args.Wnames)-1]...)
	if len(names) > 0 {
		if !o.Perm().IsDir() {
			return np.ErrNotfound
		}
		d := o.(NpObjDir)
		os, rest, r := d.Lookup(f.ctx, names)
		if r != nil || len(rest) != 0 {
			return &np.Rerror{fmt.Errorf("dir not found %v", args.Wnames).Error()}
		}
		lo = os[len(os)-1]
	}
	if args.Perm != 0 { // create?
		if !lo.Perm().IsDir() {
			return &np.Rerror{fmt.Errorf("dir not found %v", args.Wnames).Error()}
		}
		d := lo.(NpObjDir)
		name := args.Wnames[len(args.Wnames)-1]
		for {
			var ws *Watchers
			if npc.wt != nil {
				ws = npc.wt.WatchLookup(append(dname, name))
			}
			lo, r = d.Create(f.ctx, name, args.Perm, args.Mode)
			if r == nil {
				if ws != nil {
					ws.mu.Unlock()
				}
				npc.makeFid(sess, f.ctx, dname, name, lo, args.Perm.IsEphemeral())
				break
			} else {
				if npc.wt != nil && r.Error() == "Name exists" && args.Mode&np.OWATCH == np.OWATCH {
					err := ws.Watch(npc)
					if err != nil {
						return err
					}
				} else {
					if ws != nil {
						ws.mu.Unlock()
					}

					return &np.Rerror{r.Error()}
				}
			}

		}
	} else {
		npc.stats.Path(f.path)
		r = lo.Open(f.ctx, args.Mode)
		if r != nil {
			return &np.Rerror{r.Error()}
		}
	}
	switch i := lo.(type) {
	case NpObjDir:
		return np.ErrNotFile
	case NpObjFile:
		rets.Count, r = i.Write(f.ctx, args.Offset, args.Data, args.Version)
		if r != nil {
			return &np.Rerror{r.Error()}
		}
		return nil
	default:
		log.Fatalf("SetFile: obj type %T isn't NpObjDir or NpObjFile\n", o)

	}
	return nil
}
