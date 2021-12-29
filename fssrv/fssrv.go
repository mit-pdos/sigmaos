package fssrv

import (
	"fmt"
	"log"
	"runtime/debug"

	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/netsrv"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/watch"
)

//
// There is one FsServer per memfsd. The FsServer has one ProtSrv per
// 9p channel (i.e., TCP connection); each channel has one or more
// sessions (one per client fslib on the same client machine).
//

type FsServer struct {
	addr  string
	root  fs.Dir
	mkps  protsrv.MkProtServer
	stats *stats.Stats
	st    *session.SessionTable
	wt    *watch.WatchTable
	srv   *netsrv.NetServer
	pclnt *procclnt.ProcClnt
	done  bool
	ch    chan bool
	fsl   *fslib.FsLib
}

func MakeFsServer(root fs.Dir, addr string, fsl *fslib.FsLib,
	mkps protsrv.MkProtServer, pclnt *procclnt.ProcClnt,
	config repl.Config) *FsServer {
	fssrv := &FsServer{}
	fssrv.root = root
	fssrv.addr = addr
	fssrv.mkps = mkps
	fssrv.stats = stats.MkStats(fssrv.root)
	fssrv.st = session.MakeSessionTable(mkps, fssrv)
	fssrv.wt = watch.MkWatchTable()
	fssrv.srv = netsrv.MakeReplicatedNetServer(fssrv, addr, false, config)
	fssrv.pclnt = pclnt
	fssrv.ch = make(chan bool)
	fssrv.fsl = fsl
	fssrv.stats.MonitorCPUUtil()
	return fssrv
}

func (fssrv *FsServer) SetFsl(fsl *fslib.FsLib) {
	fssrv.fsl = fsl
}

func (fssrv *FsServer) Root() fs.Dir {
	return fssrv.root
}

func (fssrv *FsServer) Serve() {
	// Non-intial-named services wait on the pclnt infrastructure. Initial named waits on the channel.
	if fssrv.pclnt != nil {
		if err := fssrv.pclnt.Started(proc.GetPid()); err != nil {
			debug.PrintStack()
			log.Printf("Error Started: %v", err)
		}
		if err := fssrv.pclnt.WaitEvict(proc.GetPid()); err != nil {
			debug.PrintStack()
			log.Printf("Error WaitEvict: %v", err)
		}
	} else {
		<-fssrv.ch
	}
}

func (fssrv *FsServer) Done() {
	if fssrv.pclnt != nil {
		fssrv.pclnt.Exited(proc.GetPid(), "EVICTED")
	} else {
		if !fssrv.done {
			fssrv.done = true
			fssrv.ch <- true

		}
	}
	fssrv.stats.Done()
}

func (fssrv *FsServer) MyAddr() string {
	return fssrv.srv.MyAddr()
}

func (fssrv *FsServer) GetStats() *stats.Stats {
	return fssrv.stats
}

func (fssrv *FsServer) GetWatchTable() *watch.WatchTable {
	return fssrv.wt
}

func (fssrv *FsServer) AttachTree(uname string, aname string) (fs.Dir, fs.CtxI) {
	return fssrv.root, MkCtx(uname)
}

func (fssrv *FsServer) checkLease(sess *session.Session) error {
	fn, err := sess.LeaseName()
	if err != nil {
		return err
	}
	if fn == nil { // no lease on sess
		return nil
	}
	st, err := fssrv.fsl.Stat(np.Join(fn))
	if err != nil {
		return fmt.Errorf("checkLease failed %v", err.Error())
	}
	return sess.CheckLease(fn, st.Qid)
}

func (fssrv *FsServer) Dispatch(sid np.Tsession, msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	sess := fssrv.st.LookupInsert(sid)
	switch req := msg.(type) {
	case np.Tsetfile, np.Tgetfile, np.Tcreate, np.Topen, np.Twrite, np.Tread, np.Tremove, np.Tremovefile, np.Trenameat, np.Twstat:
		// log.Printf("%p: checkLease %v %v\n", fssrv, msg.Type(), req)
		err := fssrv.checkLease(sess)
		if err != nil {
			return nil, &np.Rerror{err.Error()}
		}
	case np.Tlease:
		reply := &np.Ropen{}
		// log.Printf("%v: %p lease %v %v %v\n", db.GetName(), fssrv, sid, msg.Type(), req)
		if err := sess.Lease(sid, req.Wnames, req.Qid); err != nil {
			return nil, &np.Rerror{err.Error()}
		}
		return *reply, nil
	case np.Tunlease:
		reply := &np.Ropen{}
		// log.Printf("%v: %p unlease %v %v %v\n", db.GetName(), fssrv, sid, msg.Type(), req)
		if err := sess.Unlease(sid, req.Wnames); err != nil {
			return nil, &np.Rerror{err.Error()}
		}
		return *reply, nil
	default:
		// log.Printf("%v: %p %v %v\n", db.GetName(), fssrv, msg.Type(), req)
	}
	fssrv.stats.StatInfo().Inc(msg.Type())
	return sess.Dispatch(msg)
}

func (fssrv *FsServer) Detach(sid np.Tsession) {
	fssrv.st.Detach(sid)
}

type Ctx struct {
	uname string
}

func MkCtx(uname string) *Ctx {
	return &Ctx{uname}
}

func (ctx *Ctx) Uname() string {
	return ctx.uname
}
