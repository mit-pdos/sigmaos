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
	Root() NpObj
	Resolver() Resolver
	Done()
}

type Resolver interface {
	Resolve(*Ctx, []string) error
}

type Ctx struct {
	uname string
	r     Resolver
}

func MkCtx(uname string, r Resolver) *Ctx {
	ctx := &Ctx{uname, r}
	return ctx
}

func (ctx *Ctx) Uname() string {
	return ctx.uname
}

type NpObj interface {
	Lookup(*Ctx, []string) ([]NpObj, []string, error)
	Qid() np.Tqid
	Perm() np.Tperm
	Size() np.Tlength
	Create(*Ctx, string, np.Tperm, np.Tmode) (NpObj, error)
	Open(*Ctx, np.Tmode) error
	ReadFile(*Ctx, np.Toffset, np.Tsize) ([]byte, error)
	WriteFile(*Ctx, np.Toffset, []byte) (np.Tsize, error)
	ReadDir(*Ctx, np.Toffset, np.Tsize) ([]*np.Stat, error)
	WriteDir(*Ctx, np.Toffset, []byte) (np.Tsize, error)
	Remove(*Ctx, string) error
	Stat(*Ctx) (*np.Stat, error)
	Rename(*Ctx, string, string) error
}

type Fid struct {
	path []string
	obj  NpObj
}

type NpConn struct {
	mu   sync.Mutex // for Fids
	conn net.Conn
	fids map[np.Tfid]*Fid
	ctx  *Ctx
	osrv NpObjSrv
}

func MakeNpConn(osrv NpObjSrv, conn net.Conn) *NpConn {
	npc := &NpConn{}
	npc.conn = conn
	npc.osrv = osrv
	npc.fids = make(map[np.Tfid]*Fid)

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
	delete(npc.fids, fid)
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
	root := npc.osrv.Root()
	npc.mu.Lock()
	if npc.ctx == nil {
		npc.ctx = &Ctx{args.Uname, npc.osrv.Resolver()}
	}
	npc.mu.Unlock()
	npc.add(args.Fid, &Fid{[]string{}, root})
	rets.Qid = root.Qid()
	return nil
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
	db.DLPrintf(npc.Addr(), "9POBJ", "Walk o %v args %v (%v)\n", f, args, len(args.Wnames))
	if len(args.Wnames) == 0 { // clone args.Fid?
		npc.add(args.NewFid, &Fid{f.path, f.obj})
	} else {
		if !f.obj.Perm().IsDir() {
			return np.ErrNotfound
		}
		if npc.ctx.r != nil {
			err := npc.ctx.r.Resolve(npc.ctx, args.Wnames)
			if err != nil {
				return &np.Rerror{err.Error()}
			}
		}
		os, rest, err := f.obj.Lookup(npc.ctx, args.Wnames)
		if err != nil {
			return np.ErrNotfound
		}
		// XXX should o be included?
		n := len(args.Wnames) - len(rest)
		p := append(f.path, args.Wnames[:n]...)
		npc.add(args.NewFid, &Fid{p, os[len(os)-1]})
		rets.Qids = makeQids(os)
	}
	return nil
}

// XXX call close? keep refcnt per obj?
func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	db.DLPrintf(npc.Addr(), "9POBJ", "Clunk %v\n", args)
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
	db.DLPrintf(npc.Addr(), "9POBJ", "Open %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf(npc.Addr(), "9POBJ", "f %v\n", f)
	err := f.obj.Open(npc.ctx, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Qid = f.obj.Qid()
	return nil
}

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	db.DLPrintf(npc.Addr(), "9POBJ", "Create %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf(npc.Addr(), "9POBJ", "f %v\n", f)

	names := []string{args.Name}
	if npc.ctx.r != nil {
		err := npc.ctx.r.Resolve(npc.ctx, names)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
	}
	if !f.obj.Perm().IsDir() {
		return &np.Rerror{fmt.Sprintf("Not a directory")}
	}
	o1, err := f.obj.Create(npc.ctx, names[0], args.Perm, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}

	npc.add(args.Fid, &Fid{append(f.path, names[0]), o1})
	rets.Qid = o1.Qid()
	return nil
}

func (npc *NpConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (npc *NpConn) readDir(o NpObj, args np.Tread, rets *np.Rread) *np.Rerror {
	var dirents []*np.Stat
	var err error
	if o.Size() > 0 && args.Offset >= np.Toffset(o.Size()) {
		dirents = []*np.Stat{}
	} else {
		dirents, err = o.ReadDir(npc.ctx, args.Offset, args.Count)

	}
	b, err := npcodec.Dir2Byte(args.Offset, args.Count, dirents)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = b
	return nil
}

// XXX check for offset > len here?
func (npc *NpConn) readFile(o NpObj, args np.Tread, rets *np.Rread) *np.Rerror {
	b, err := o.ReadFile(npc.ctx, args.Offset, args.Count)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = b
	return nil
}

func (npc *NpConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	db.DLPrintf(npc.Addr(), "9POBJ", "Read %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf(npc.Addr(), "9POBJ", "ReadFid %v %v\n", args, f)
	if f.obj.Perm().IsDir() {
		return npc.readDir(f.obj, args, rets)
	} else {
		return npc.readFile(f.obj, args, rets)
	}
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	db.DLPrintf(npc.Addr(), "9POBJ", "Write %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf(npc.Addr(), "9POBJ", "Write f %v\n", f)
	var err error
	cnt := np.Tsize(0)
	if f.obj.Perm().IsDir() {
		cnt, err = f.obj.WriteDir(npc.ctx, args.Offset, args.Data)
	} else {
		cnt, err = f.obj.WriteFile(npc.ctx, args.Offset, args.Data)
	}
	rets.Count = np.Tsize(cnt)
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
		npc.osrv.Done()
		return nil
	}
	db.DLPrintf(npc.Addr(), "9POBJ", "Remove f %v\n", f)
	err := f.obj.Remove(npc.ctx, f.path[len(f.path)-1])
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.del(args.Fid)
	return nil
}

func (npc *NpConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DLPrintf(npc.Addr(), "9POBJ", "Stat %v\n", f)
	st, err := f.obj.Stat(npc.ctx)
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
	db.DLPrintf(npc.Addr(), "9POBJ", "Wstat %v %v\n", f, args)
	if args.Stat.Name != "" {
		err := f.obj.Rename(npc.ctx, f.path[len(f.path)-1], args.Stat.Name)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		f.path = append(f.path[:len(f.path)-1], np.Split(args.Stat.Name)...)
	}
	// XXX ignore other Wstat for now
	return nil
}
