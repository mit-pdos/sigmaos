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
	MakeObj([]string, np.Tperm, NpObj) NpObj
	Root() NpObj
	Done()
}

type NpObj interface {
	Lookup([]string) (NpObj, error)
	ReadDir() ([]*np.Stat, error)
	Qid() np.Tqid
	Perm() np.Tperm
	Size() np.Tlength
	Path() []string
	Create(string, np.Tperm, np.Tmode) (NpObj, error)
	ReadFile(np.Toffset, np.Tsize) ([]byte, error)
	WriteFile(np.Toffset, []byte) (np.Tsize, error)
	WriteDir(np.Toffset, []byte) (np.Tsize, error)
	Remove() error
	Stat() (*np.Stat, error)
	Wstat(*np.Stat) error
}

type NpConn struct {
	mu    sync.Mutex // for Fids
	conn  net.Conn
	fids  map[np.Tfid]NpObj
	uname string
	osrv  NpObjSrv
}

func MakeNpConn(osrv NpObjSrv, conn net.Conn) *NpConn {
	npc := &NpConn{}
	npc.conn = conn
	npc.osrv = osrv
	npc.fids = make(map[np.Tfid]NpObj)
	return npc
}

func (npc *NpConn) lookup(fid np.Tfid) (NpObj, bool) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	o, ok := npc.fids[fid]
	return o, ok
}

func (npc *NpConn) add(fid np.Tfid, o NpObj) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	npc.fids[fid] = o
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
	db.DPrintf("Attach %v\n", args)
	npc.uname = args.Uname
	root := npc.osrv.Root()
	npc.add(args.Fid, root)
	rets.Qid = root.Qid()
	return nil
}

func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("Walk o %v args %v\n", o, args)
	if len(args.Wnames) == 0 { // clone args.Fid?
		npc.add(args.NewFid, o)
	} else {
		if !o.Perm().IsDir() {
			return np.ErrNotfound
		}
		o1, err := o.Lookup(args.Wnames)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		npc.add(args.NewFid, o1)
		rets.Qids = []np.Tqid{o1.Qid()}
	}

	return nil
}

func (npc *NpConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	db.DPrintf("Clunk %v\n", args)
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
	db.DPrintf("Open %v\n", args)
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("o %v\n", o)
	rets.Qid = o.Qid()
	return nil
}

// XXX directories don't work: there is a fake directory, when trying
// to read it we get an error.  Maybe create . or .. in the directory
// args.Name, to force the directory into existence
func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	db.DPrintf("Create %v\n", args)
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("o %v\n", o)
	if !o.Perm().IsDir() {
		return &np.Rerror{fmt.Sprintf("Not a directory")}
	}
	if args.Perm.IsDir() { // fake a directory?
		o1 := npc.osrv.MakeObj(append(o.Path(), args.Name), np.DMDIR, o)
		npc.add(args.Fid, o1)
		rets.Qid = o1.Qid()
		return nil
	}
	o1, err := o.Create(args.Name, args.Perm, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.add(args.Fid, o1)
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
		dirents, err = o.ReadDir()

	}
	b, err := npcodec.Dir2Byte(args.Offset, args.Count, dirents)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = b
	return nil
}

func (npc *NpConn) readFile(o NpObj, args np.Tread, rets *np.Rread) *np.Rerror {
	b, err := o.ReadFile(args.Offset, args.Count)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = b
	return nil
}

func (npc *NpConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	db.DPrintf("Read %v\n", args)
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("ReadFid %v %v\n", args, o)
	if o.Perm().IsDir() {
		return npc.readDir(o, args, rets)
	} else {
		return npc.readFile(o, args, rets)
	}
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	db.DPrintf("Write %v\n", args)
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("Write o %v\n", o)
	var err error
	cnt := np.Tsize(0)
	if o.Perm().IsDir() {
		cnt, err = o.WriteDir(args.Offset, args.Data)
	} else {
		cnt, err = o.WriteFile(args.Offset, args.Data)
	}
	rets.Count = np.Tsize(cnt)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	return nil
}

func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if len(o.Path()) == 0 { // exit?
		npc.osrv.Done()
		return nil
	}
	db.DPrintf("Remove o %v\n", o)
	err := o.Remove()
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.del(args.Fid)
	return nil
}

func (npc *NpConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("Stat %v\n", o)
	st, err := o.Stat()
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
	db.DPrintf("Wstat %v\n", f)
	// XXX ignore Wstat for now
	return nil
}
