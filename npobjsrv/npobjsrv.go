package npobjsrv

import (
	"fmt"
	"net"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type NpObjSrv interface {
	// Maybe pass uname to RootAttach()
	RootAttach(string) (NpObj, CtxI)
	Done()
}

type CtxI interface {
	Uname() string
}

type NpObj interface {
	Lookup(CtxI, []string) ([]NpObj, []string, error)
	Qid() np.Tqid
	Perm() np.Tperm
	Version() np.TQversion
	Size() np.Tlength
	Create(CtxI, string, np.Tperm, np.Tmode) (NpObj, error)
	Open(CtxI, np.Tmode) error
	ReadFile(CtxI, np.Toffset, np.Tsize) ([]byte, error)
	WriteFile(CtxI, np.Toffset, []byte) (np.Tsize, error)
	ReadDir(CtxI, np.Toffset, np.Tsize) ([]*np.Stat, error)
	WriteDir(CtxI, np.Toffset, []byte) (np.Tsize, error)
	Remove(CtxI, string) error
	Stat(CtxI) (*np.Stat, error)
	Rename(CtxI, string, string) error
}

type Fid struct {
	path []string
	obj  NpObj
	vers np.TQversion
	ctx  CtxI
}

func (f *Fid) Write(off np.Toffset, b []byte) (np.Tsize, error) {
	if f.obj.Perm().IsDir() {
		return f.obj.WriteDir(f.ctx, off, b)
	} else {
		return f.obj.WriteFile(f.ctx, off, b)
	}
}

func (f *Fid) readDir(off np.Toffset, count np.Tsize, rets *np.Rread) *np.Rerror {
	var dirents []*np.Stat
	var err error
	if f.obj.Size() > 0 && off >= np.Toffset(f.obj.Size()) {
		dirents = []*np.Stat{}
	} else {
		dirents, err = f.obj.ReadDir(f.ctx, off, count)

	}
	b, err := npcodec.Dir2Byte(off, count, dirents)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = b
	return nil
}

// XXX check for offset > len here?
func (f *Fid) readFile(off np.Toffset, count np.Tsize, rets *np.Rread) *np.Rerror {
	b, err := f.obj.ReadFile(f.ctx, off, count)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = b
	return nil
}

func (f *Fid) Read(off np.Toffset, count np.Tsize, rets *np.Rread) *np.Rerror {
	if f.obj.Perm().IsDir() {
		return f.readDir(off, count, rets)
	} else {
		return f.readFile(off, count, rets)
	}
}

type Watch struct {
	ch chan bool
	n  int
}

func mkWatch() *Watch {
	return &Watch{make(chan bool), 1}
}

type NpConn struct {
	mu        sync.Mutex // for Fids
	conn      net.Conn
	fids      map[np.Tfid]*Fid
	osrv      NpObjSrv
	ephemeral map[NpObj]*Fid
	watches   map[string]*Watch
}

func MakeNpConn(osrv NpObjSrv, conn net.Conn) *NpConn {
	npc := &NpConn{}
	npc.conn = conn
	npc.osrv = osrv
	npc.fids = make(map[np.Tfid]*Fid)
	npc.ephemeral = make(map[NpObj]*Fid)
	npc.watches = make(map[string]*Watch)
	return npc
}

func (npc *NpConn) Addr() string {
	return npc.conn.LocalAddr().String()
}

func (npc *NpConn) lookup(fid np.Tfid) (*Fid, bool) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	f, ok := npc.fids[fid]
	return f, ok
}

func (npc *NpConn) add(fid np.Tfid, f *Fid) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	npc.fids[fid] = f
}

func (npc *NpConn) del(fid np.Tfid) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	o := npc.fids[fid].obj
	delete(npc.fids, fid)
	delete(npc.ephemeral, o)
}

func (npc *NpConn) Watch(path []string) {
	p := np.Join(path)
	npc.mu.Lock()
	w, ok := npc.watches[p]
	if !ok {
		w = mkWatch()
		npc.watches[p] = w
	} else {
		w.n += 1
	}
	npc.mu.Unlock()
	db.DLPrintf("9POBJ", "Watch %v %v\n", p, w)
	<-w.ch
	db.DLPrintf("9POBJ", "Watch success %v %v\n", p, w)
	npc.mu.Lock()
	w.n -= 1
	if w.n == 0 {
		db.DLPrintf("9POBJ", "Watch delete %v %v\n", p, w)
		delete(npc.watches, p)
	}
	npc.mu.Unlock()
}

func (npc *NpConn) wakeupWatch(path []string) {
	p := np.Join(path)

	npc.mu.Lock()
	w, ok := npc.watches[p]
	npc.mu.Unlock()
	if !ok {
		return
	}
	db.DLPrintf("9POBJ", "wakeupWatch %v %v\n", p, w)
	w.ch <- true
}

func (npc *NpConn) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (npc *NpConn) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.ErrUnknownMsg
}

func (npc *NpConn) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	root, ctx := npc.osrv.RootAttach(args.Uname)
	npc.add(args.Fid, &Fid{[]string{}, root, 0, ctx})
	rets.Qid = root.Qid()
	return nil
}

func (npc *NpConn) Detach() {
	db.DLPrintf("9POBJ", "Detach %v %v\n", npc.ephemeral, npc.watches)

	// Delete ephemeral files created on this connection
	for o, f := range npc.ephemeral {
		o.Remove(f.ctx, f.path[len(f.path)-1])
	}

	// Cleanup threads waiting for a watch on this connection
	for p, _ := range npc.watches {
		npc.wakeupWatch(np.Split(p))
	}
}

func makeQids(os []NpObj) []np.Tqid {
	var qids []np.Tqid
	for _, o := range os {
		qids = append(qids, o.Qid())
	}
	return qids
}

func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf("9POBJ", "Walk o %v args %v (%v)\n", f, args, len(args.Wnames))
	if len(args.Wnames) == 0 { // clone args.Fid?
		npc.add(args.NewFid, &Fid{f.path, f.obj, f.obj.Version(), f.ctx})
	} else {
		if !f.obj.Perm().IsDir() {
			return np.ErrNotfound
		}
		os, rest, err := f.obj.Lookup(f.ctx, args.Wnames)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		n := len(args.Wnames) - len(rest)
		p := append(f.path, args.Wnames[:n]...)
		lo := os[len(os)-1]
		npc.add(args.NewFid, &Fid{p, lo, lo.Version(), f.ctx})
		rets.Qids = makeQids(os)
	}
	return nil
}

// XXX call close? keep refcnt per obj?
func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	db.DLPrintf("9POBJ", "Clunk %v\n", args)
	_, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	npc.mu.Lock()
	delete(npc.fids, args.Fid)
	npc.mu.Unlock()
	return nil
}

func (npc *NpConn) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	db.DLPrintf("9POBJ", "Open %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf("9POBJ", "f %v\n", f)
	err := f.obj.Open(f.ctx, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Qid = f.obj.Qid()
	return nil
}

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	db.DLPrintf("9POBJ", "Create %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf("9POBJ", "f %v\n", f)

	names := []string{args.Name}
	if !f.obj.Perm().IsDir() {
		return &np.Rerror{fmt.Sprintf("Not a directory")}
	}
	for {
		o1, err := f.obj.Create(f.ctx, names[0], args.Perm, args.Mode)
		db.DLPrintf("9POBJ", "Create %v %v %v\n", names[0], o1, err)
		if err == nil {
			nf := &Fid{append(f.path, names[0]), o1, o1.Version(), f.ctx}
			if args.Perm.IsEphemeral() {
				npc.mu.Lock()
				npc.ephemeral[o1] = nf
				npc.mu.Unlock()
			}
			npc.add(args.Fid, nf)
			rets.Qid = o1.Qid()
			break
		} else {
			if err.Error() == "Name exists" && args.Mode&np.OCEXEC == np.OCEXEC { // retry?
				p := append(f.path, names[0])
				db.DLPrintf("9POBJ", "Watch %v\n", p)
				npc.Watch(p)
			} else {
				return &np.Rerror{err.Error()}
			}
		}
	}
	return nil
}

func (npc *NpConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (npc *NpConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	db.DLPrintf("9POBJ", "Read %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf("9POBJ", "ReadFid %v %v\n", args, f)
	return f.Read(args.Offset, args.Count, rets)
}

func (npc *NpConn) ReadV(args np.Treadv, rets *np.Rread) *np.Rerror {
	db.DLPrintf("9POBJ", "ReadV %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if f.vers != f.obj.Version() {
		return &np.Rerror{"Version mismatch"}
	}
	return f.Read(args.Offset, args.Count, rets)
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	db.DLPrintf("9POBJ", "Write %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	var err error
	rets.Count, err = f.Write(args.Offset, args.Data)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (npc *NpConn) WriteV(args np.Twritev, rets *np.Rwrite) *np.Rerror {
	db.DLPrintf("9POBJ", "WriteV %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if f.vers != f.obj.Version() {
		return &np.Rerror{"Version mismatch"}
	}
	var err error
	rets.Count, err = f.Write(args.Offset, args.Data)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if len(f.path) == 0 { // exit?
		db.DLPrintf("9POBJ", "Done\n")
		npc.osrv.Done()
		return nil
	}
	db.DLPrintf("9POBJ", "Remove f %v\n", f)
	err := f.obj.Remove(f.ctx, f.path[len(f.path)-1])
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.wakeupWatch(f.path)
	npc.del(args.Fid)
	return nil
}

func (npc *NpConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf("9POBJ", "Stat %v\n", f)
	st, err := f.obj.Stat(f.ctx)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Stat = *st
	return nil
}

func (npc *NpConn) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf("9POBJ", "Wstat %v %v\n", f, args)
	if args.Stat.Name != "" {
		err := f.obj.Rename(f.ctx, f.path[len(f.path)-1], args.Stat.Name)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		f.path = append(f.path[:len(f.path)-1], np.Split(args.Stat.Name)...)
	}
	// XXX ignore other Wstat for now
	return nil
}
