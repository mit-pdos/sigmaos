package nps3

import (
	"fmt"
	"net"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/npsrv"
)

var bucket = "9ps3"

const (
	CHUNKSZ = 8192
)

type NpConn struct {
	mu    sync.Mutex // for Fids
	conn  net.Conn
	fids  map[np.Tfid]*Obj
	uname string
	nps3  *Nps3
}

func makeNpConn(nps3 *Nps3, conn net.Conn) *NpConn {
	npc := &NpConn{}
	npc.conn = conn
	npc.nps3 = nps3
	npc.fids = make(map[np.Tfid]*Obj)
	return npc
}

func (npc *NpConn) lookup(fid np.Tfid) (*Obj, bool) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	o, ok := npc.fids[fid]
	return o, ok
}

func (npc *NpConn) add(fid np.Tfid, o *Obj) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	npc.fids[fid] = o
}

func (npc *NpConn) del(fid np.Tfid) {
	npc.mu.Lock()
	defer npc.mu.Unlock()
	delete(npc.fids, fid)
}

func (nps3 *Nps3) Connect(conn net.Conn) npsrv.NpAPI {
	clnt := makeNpConn(nps3, conn)
	return clnt
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
	o := npc.nps3.makeObj([]string{}, np.DMDIR)
	npc.add(args.Fid, o)
	rets.Qid = o.qid()
	return nil
}

// XXX quite expensive: each time walk the files in a directory, we
// lookup the directory again.
func (npc *NpConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	db.DPrintf("Walk %v\n", args)
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("Walk o %v\n", o)
	if len(args.Wnames) == 0 { // clone args.Fid?
		npc.add(args.NewFid, o)
	} else {
		if o.t != np.DMDIR {
			return np.ErrNotfound
		}
		_, err := o.readDir()
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		var key string
		if len(o.key) > 0 {
			key = np.Join(o.key) + "/" + np.Join(args.Wnames)
		} else {
			key = np.Join(args.Wnames)
		}
		o1, ok := npc.nps3.lookupKey(key)
		if !ok {
			return np.ErrNotfound
		}
		npc.add(args.NewFid, o1)
		rets.Qids = []np.Tqid{o1.qid()}
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
	rets.Qid = o.qid()
	return nil
}

// XXX directories don't work: there is a fake directory, when trying
// to read it we get an error.
func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	db.DPrintf("Create %v\n", args)
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("o %v\n", o)
	if o.t != np.DMDIR {
		return &np.Rerror{fmt.Sprintf("Not a directory")}
	}
	if args.Perm&np.DMDIR == np.DMDIR { // fake a directory?
		o1 := o.nps3.makeObj(append(o.key, args.Name), np.DMDIR)
		npc.add(args.Fid, o1)
		rets.Qid = o1.qid()
		return nil
	}
	o1, err := o.Create(args.Name, args.Perm, args.Mode)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	npc.add(args.Fid, o1)
	rets.Qid = o1.qid()
	return nil
}

func (npc *NpConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (npc *NpConn) readDir(o *Obj, args np.Tread, rets *np.Rread) *np.Rerror {
	dirents, err := o.readDir()
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	b, err := npcodec.Dir2Byte(args.Offset, args.Count, dirents)
	if err != nil {
		return &np.Rerror{err.Error()}
	}
	rets.Data = b
	return nil
}

func (npc *NpConn) readFile(o *Obj, args np.Tread, rets *np.Rread) *np.Rerror {
	b, err := o.readFile(int(args.Offset), int(args.Count))
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
	switch o.t {
	case np.DMDIR:
		return npc.readDir(o, args, rets)
	case 0:
		return npc.readFile(o, args, rets)
	default:
		return np.ErrNotSupported
	}
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	db.DPrintf("Write %v\n", args)
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("Write o %v\n", o)
	switch o.t {
	case np.DMDIR:
		// sub directories will be implicitly created; fake write
		rets.Count = np.Tsize(len(args.Data))
		return nil
	case 0:
		cnt, err := o.writeFile(int(args.Offset), args.Data)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		rets.Count = np.Tsize(cnt)
	default:
		return np.ErrNotSupported
	}
	return nil
}

func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("Remove %v\n", o)
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
	rets.Stat = *o.stat()
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
