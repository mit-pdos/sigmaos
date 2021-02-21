package nps3

import (
	"log"
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
		log.Printf("key: %v\n", key)
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

func (npc *NpConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	db.DPrintf("Create %v\n", args)
	o, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("o %v\n", o)
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
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("ReadFid %v %v\n", args, f)
	if f.t == np.DMDIR {
		return npc.readDir(f, args, rets)
	} else if f.t == 0 {
		return npc.readFile(f, args, rets)
	} else {
		return np.ErrNotSupported
	}
}

func (npc *NpConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	db.DPrintf("Write %v\n", args)
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("Write f %v\n", f)
	n := np.Tsize(0)
	rets.Count = n
	return nil
}

func (npc *NpConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	f, ok := npc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("Remove %v\n", f)
	return np.ErrNotSupported
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
