package schedd

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/npsrv"
)

type objT uint16

const (
	ROOT   objT = 0
	DEV    objT = 1
	LAMBDA objT = 2
	FIELD  objT = 3
)

func (t objT) mode() np.Tperm {
	switch t {
	case ROOT:
		return np.DMDIR
	case DEV:
		return np.DMDEVICE
	case LAMBDA:
		return np.DMDIR
	default:
		return np.Tperm(0)
	}
}

type Obj struct {
	t objT
	l *Lambda
	f string
}

func (o *Obj) String() string {
	return fmt.Sprintf("t %v l %v f %v", o.t, o.l, o.f)
}

var rootO = &Obj{ROOT, nil, ""}
var devO = &Obj{DEV, nil, ""}

func (o *Obj) qid() np.Tqid {
	switch o.t {
	case ROOT:
		return np.MakeQid(np.Qtype(np.DMDIR>>np.QTYPESHIFT),
			np.TQversion(0), np.Tpath(0))
	case DEV:
		return np.MakeQid(np.Qtype(np.DMDEVICE>>np.QTYPESHIFT),
			np.TQversion(0), np.Tpath(1))
	case LAMBDA:
		return o.l.qid(true)
	default:
		return o.l.qid(false)
	}
}

func (o *Obj) stat() *np.Stat {
	st := &np.Stat{}
	st.Uid = "sched"
	st.Gid = "sched"
	// XXX not every field of attr should 0777
	st.Mode = np.Tperm(0777) | o.t.mode()
	st.Mtime = uint32(time.Now().Unix())
	switch o.t {
	case ROOT:
		st.Qid = rootO.qid()
	case DEV:
		st.Qid = devO.qid()
		st.Name = "dev"
	case LAMBDA:
		st = o.l.stat("lambda")
	case FIELD:
		st = o.l.stat(o.f)
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
	sc.Fids[args.Fid] = &Obj{ROOT, nil, ""}
	rets.Qid = rootO.qid()
	return nil
}

func (sc *SchedConn) walkField(l *Lambda, args np.Twalk, rets *np.Rwalk) *np.Rerror {
	name := args.Wnames[0]
	r, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsUpper(r) {
		return &np.Rerror{fmt.Sprintf("Lower-case field %v", name)}
	}
	v := reflect.ValueOf(Lambda{})
	t := v.Type()
	_, ok := t.FieldByName(name)
	if !ok {
		return &np.Rerror{fmt.Sprintf("Unknown field %v", name)}
	}
	o1 := &Obj{FIELD, l, name}
	sc.add(args.NewFid, o1)
	rets.Qids = append(rets.Qids, o1.qid())
	return nil
}

func (sc *SchedConn) Walk(args np.Twalk, rets *np.Rwalk) *np.Rerror {
	db.DPrintf("Walk %v\n", args)
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if len(args.Wnames) == 0 { // clone args.Fid?
		o, ok := sc.lookup(args.Fid)
		if !ok {
			return np.ErrUnknownfid
		}
		sc.add(args.NewFid, o)
	} else if args.Wnames[0] == "dev" {
		sc.add(args.NewFid, devO)
		rets.Qids = []np.Tqid{devO.qid()}
	} else if o.t == LAMBDA {
		return sc.walkField(o.l, args, rets)
	} else {
		l := sc.sched.findLambda(args.Wnames[0])
		if l == nil {
			return &np.Rerror{fmt.Sprintf("Unknown lambda %v", args.Wnames[0])}
		}
		o1 := &Obj{LAMBDA, l, ""}
		sc.add(args.NewFid, o1)
		rets.Qids = []np.Tqid{o1.qid()}
		if len(args.Wnames) > 1 {
			args.Wnames = args.Wnames[1:]
			return sc.walkField(l, args, rets)
		}
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
	sc.add(args.Fid, &Obj{LAMBDA, l, ""})
	rets.Qid = l.qid(true)
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

func (sc *SchedConn) readField(o *Obj, args np.Tread, rets *np.Rread) *np.Rerror {
	if args.Offset != 0 {
		return nil
	}
	b, err := o.l.readField(o.f)
	if err != nil {
		return &np.Rerror{fmt.Sprintf("Read field %v error %v", o.f, err)}
	}
	rets.Data = b
	return nil
}

func (sc *SchedConn) Read(args np.Tread, rets *np.Rread) *np.Rerror {
	db.DPrintf("Read %v\n", args)
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	db.DPrintf("ReadObj %v %v\n", args, o)
	if o.t == ROOT {
		dir := sc.ls()
		b, err := npcodec.Dir2Byte(args.Offset, args.Count, dir)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		rets.Data = b
	} else if o.t == DEV {
		return np.ErrNotSupported
	} else if o.t == LAMBDA {
		dir := o.l.ls()
		b, err := npcodec.Dir2Byte(args.Offset, args.Count, dir)
		if err != nil {
			return &np.Rerror{err.Error()}
		}
		rets.Data = b
	} else if o.t == FIELD {
		return sc.readField(o, args, rets)
	}
	return nil
}

func (sc *SchedConn) devWrite(t string) *np.Rerror {
	db.DPrintf("Write dev %v\n", t)
	if strings.HasPrefix(t, "SwapExitDependencies") {
		sc.sched.swapExitDependencies(
			strings.Split(
				strings.TrimSpace(t[len("SwapExitDependencies "):]),
				" ",
			),
		)
	} else {
		return np.ErrNotSupported
	}
	return nil
}

func (sc *SchedConn) writeField(o *Obj, args np.Twrite, rets *np.Rwrite) *np.Rerror {
	if args.Offset != 0 {
		return nil
	}
	err := o.l.writeField(o.f, args.Data)
	if err != nil {
		return &np.Rerror{fmt.Sprintf("Write field %v error %v", o.f, err)}
	}
	rets.Count = np.Tsize(len(args.Data))
	return nil
}

func (sc *SchedConn) Write(args np.Twrite, rets *np.Rwrite) *np.Rerror {
	db.DPrintf("Write %v\n", args)
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	n := np.Tsize(0)
	switch o.t {
	case ROOT:
		return np.ErrNowrite
	case DEV:
		r := sc.devWrite(string(args.Data))
		if r != nil {
			return r
		}
		n = np.Tsize(len(args.Data))
	case LAMBDA:
		if o.l.Status == "Init" {
			err := o.l.init(args.Data)
			if err != nil {
				return &np.Rerror{err.Error()}
			}
			sc.sched.spawn(o.l)
			n = np.Tsize(len(args.Data))
			db.DPrintf("initl %v\n", o.l)
		} else if o.l.Status == "Running" { // a continuation
			err := o.l.continueing(args.Data)
			if err != nil {
				return &np.Rerror{err.Error()}
			}
			n = np.Tsize(len(args.Data))
			db.DPrintf("conitnuel %v\n", o.l)
		} else {
			return &np.Rerror{fmt.Sprintf("Lambda already running")}
		}
	default:
		return sc.writeField(o, args, rets)
	}
	rets.Count = n
	return nil
}

// like kill?
func (sc *SchedConn) Remove(args np.Tremove, rets *np.Rremove) *np.Rerror {
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	if o.t == ROOT {
		sc.sched.exit()
	} else {
		return np.ErrNotSupported
	}
	return nil
}

func (sc *SchedConn) Stat(args np.Tstat, rets *np.Rstat) *np.Rerror {
	o, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	rets.Stat = *o.stat()
	if o.t == ROOT {
		rets.Stat.Length = npcodec.DirSize(sc.ls())
	}
	return nil
}

func (sc *SchedConn) Wstat(args np.Twstat, rets *np.Rwstat) *np.Rerror {
	_, ok := sc.lookup(args.Fid)
	if !ok {
		return np.ErrUnknownfid
	}
	// XXX ignore Wstat for now
	return nil
}
