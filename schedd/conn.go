package schedd

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/npsrv"
)

type Obj struct {
	t np.Tperm
	l *Lambda
}

var rootO = &Obj{np.DMDIR, nil}
var devO = &Obj{np.DMDEVICE, nil}

func (o *Obj) qid() np.Tqid {
	if o.t == np.DMDIR {
		return np.MakeQid(np.Qtype(np.DMDIR>>np.QTYPESHIFT), np.TQversion(0),
			np.Tpath(0))
	} else if o.t == np.DMDEVICE {
		return np.MakeQid(np.Qtype(np.DMDEVICE>>np.QTYPESHIFT), np.TQversion(0),
			np.Tpath(1))
	} else {
		return o.l.qid()
	}
}

func (o *Obj) stat() *np.Stat {
	st := &np.Stat{}
	st.Uid = "sched"
	st.Gid = "sched"
	st.Mode = np.Tperm(0777) | o.t
	st.Mtime = uint32(time.Now().Unix())
	if o.t == np.DMDIR {
		st.Qid = rootO.qid()
	} else if o.t == np.DMDEVICE {
		st.Qid = devO.qid()
		st.Name = "dev"
	} else {
		st = o.l.stat()
	}
	return st
}

type SchedConn struct {
	mu    sync.Mutex // for Fids
	conn  net.Conn
	Fids  map[np.Tfid]*Obj
	uname string
	sched *Sched
}

func makeSchedConn(sched *Sched, conn net.Conn) *SchedConn {
	sc := &SchedConn{}
	sc.conn = conn
	sc.sched = sched
	sc.Fids = make(map[np.Tfid]*Obj)
	return sc
}

func (sc *SchedConn) lookup(fid np.Tfid) (*Obj, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	f, ok := sc.Fids[fid]
	return f, ok
}

func (sc *SchedConn) add(fid np.Tfid, o *Obj) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.Fids[fid] = o
}

func (sd *Sched) Connect(conn net.Conn) npsrv.NpAPI {
	clnt := makeSchedConn(sd, conn)
	return clnt
}

func (sc *SchedConn) Version(args np.Tversion, rets *np.Rversion) *np.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (sc *SchedConn) Auth(args np.Tauth, rets *np.Rauth) *np.Rerror {
	return np.ErrUnknownMsg
}

func (sc *SchedConn) Attach(args np.Tattach, rets *np.Rattach) *np.Rerror {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	db.DPrintf("Attach %v\n", args)
	sc.uname = args.Uname
	sc.Fids[args.Fid] = &Obj{np.DMDIR, nil}
	rets.Qid = rootO.qid()
	return nil
}

func (sc *SchedConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	db.DPrintf("Walk %v\n", args)
	if len(args.Wnames) == 0 {
		sc.add(args.NewFid, rootO)
	} else if args.Wnames[0] == "dev" {
		sc.add(args.NewFid, devO)
		rets.Qids = []np.Tqid{devO.qid()}
	} else {
		var qids []np.Tqid
		l := sc.sched.findLambda(args.Wnames[0])
		if l == nil {
			return &np.Rerror{fmt.Sprintf("Unknown name %v", args.Wnames[0])}
		}
		sc.add(args.NewFid, &Obj{0, l})
		rets.Qids = append(qids, l.qid())
	}
	return nil
}

func (sc *SchedConn) Clunk(args np.Tclunk, rets *np.Rclunk) *np.Rerror {
	db.DPrintf("Clunk %v\n", args)
	_, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	sc.mu.Lock()
	delete(sc.Fids, args.Fid)
	sc.mu.Unlock()
	return nil
}

func (sc *SchedConn) Open(args np.Topen, rets *np.Ropen) *np.Rerror {
	db.DPrintf("Open %v\n", args)
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	rets.Qid = o.qid()
	return nil
}

func (sc *SchedConn) Create(args np.Tcreate, rets *np.Rcreate) *np.Rerror {
	db.DPrintf("Create %v\n", args)
	_, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	l := makeLambda(sc.sched, args.Name)
	sc.add(args.Fid, &Obj{0, l})
	rets.Qid = l.qid()
	return nil
}

func (sc *SchedConn) Flush(args np.Tflush, rets *np.Rflush) *np.Rerror {
	return nil
}

func (sc *SchedConn) ls() []*np.Stat {
	dir := sc.sched.ps()
	dir = append([]*np.Stat{devO.stat()}, dir...)
	db.DPrintf("ls: %v\n", dir)
	return dir
}

func (sc *SchedConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	db.DPrintf("Read %v\n", args)
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if o.t == np.DMDIR { // root directory
		dir := sc.ls()
		b, err := npcodec.Dir2Buf(args.Offset, args.Count, dir)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		rets.Data = b
	} else if o.t == np.DMDEVICE { // dev
		return np.ErrNotSupported
	} else {
		if args.Offset == 0 {
			o.l.waitFor()
			rets.Data = []byte(o.l.exitStatus)
		}
	}
	return nil
}

func (sc *SchedConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	db.DPrintf("Write %v\n", args)
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	n := np.Tsize(0)
	if o.t == np.DMDIR {
		return np.ErrNowrite
	} else if o.t == np.DMDEVICE {
		t := string(args.Data)
		db.DPrintf("Write dev %v\n", t)
		if strings.HasPrefix(t, "Started") {
			sc.sched.started(t[len("Started "):])
		} else if strings.HasPrefix(t, "Exiting") {
			sc.sched.exiting(strings.TrimSpace(t[len("Exiting "):]))
		} else if strings.HasPrefix(t, "SwapExitDependencies") {
			sc.sched.swapExitDependencies(
				strings.Split(
					strings.TrimSpace(t[len("SwapExitDependencies "):]),
					" ",
				),
			)
		} else if strings.HasPrefix(t, "Exit") { // must go after Exiting
			sc.sched.exit()
		} else {
			return np.ErrNotSupported
		}
		n = np.Tsize(len(args.Data))
	} else {
		err := o.l.initLambda(args.Data)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		sc.sched.spawn(o.l)
		n = np.Tsize(len(args.Data))
		db.DPrintf("initl %v\n", o.l)
	}
	rets.Count = n
	return nil
}

// like kill?
func (sc *SchedConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	_, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	return np.ErrNotSupported
	return nil
}

func (sc *SchedConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	rets.Stat = *o.stat()
	if o.t == np.DMDIR {
		rets.Stat.Length = npcodec.DirSize(sc.ls())
	}
	return nil
}

func (sc *SchedConn) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	return np.ErrNotSupported
}
