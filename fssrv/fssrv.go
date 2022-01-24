package fssrv

import (
	"log"
	"runtime/debug"

	// db "ulambda/debug"
	"ulambda/ctx"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/netsrv"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/sesscond"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/watch"
)

//
// There is one FsServer per server. The FsServer has one ProtSrv per
// 9p channel (i.e., TCP connection). Each channel may multiplex
// several users/clients.
//
// FsServer has a table with all sess conds in use so that it can
// unblock threads that are waiting in a sess cond when a session
// closes.
//

type FsServer struct {
	addr  string
	root  fs.Dir
	mkps  protsrv.MkProtServer
	stats *stats.Stats
	st    *session.SessionTable
	sct   *sesscond.SessCondTable
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
	fssrv.sct = sesscond.MakeSessCondTable(fssrv.st)
	fssrv.wt = watch.MkWatchTable(fssrv.sct)
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

func (fssrv *FsServer) GetSessCondTable() *sesscond.SessCondTable {
	return fssrv.sct
}

func (fssrv *FsServer) Root() fs.Dir {
	return fssrv.root
}

// The server using fssrv is ready to take requests. Keep serving
// until fssrv is told to stop using Done().
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

// The server using fssrv is done; exit.
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

func (fssrv *FsServer) AttachTree(uname string, aname string, sessid np.Tsession) (fs.Dir, fs.CtxI) {
	return fssrv.root, ctx.MkCtx(uname, sessid, fssrv.sct)
}

func (fssrv *FsServer) Process(fc *np.Fcall, replies chan *np.Fcall) {
	sess := fssrv.st.Alloc(fc.Session)
	reply, rerror := fssrv.fenceSession(sess, fc.Msg)
	if rerror != nil {
		reply = rerror
	}
	if reply != nil {
		fssrv.sendReply(fc.Tag, reply, replies)
		return
	}
	fssrv.stats.StatInfo().Inc(fc.Msg.Type())
	go fssrv.serve(sess, fc, replies)
}

// Register lease, unregister lease, and check lease
func (fssrv *FsServer) fenceSession(sess *session.Session, msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	switch req := msg.(type) {
	case np.Tsetfile, np.Tgetfile, np.Tcreate, np.Topen, np.Twrite, np.Tread, np.Tremove, np.Tremovefile, np.Trenameat, np.Twstat:
		// log.Printf("%p: checkLease %v %v\n", fssrv, msg.Type(), req)
		err := sess.CheckLeases(fssrv.fsl)
		if err != nil {
			return nil, &np.Rerror{err.Error()}
		}
	case np.Tlease:
		reply := &np.Ropen{}
		// log.Printf("%v: %p reglease %v %v %v\n", db.GetName(), fssrv, sid, msg.Type(), req)
		if err := sess.Lease(req.Wnames, req.Qid); err != nil {
			return nil, &np.Rerror{err.Error()}
		}
		return reply, nil
	case np.Tunlease:
		reply := &np.Ropen{}
		// log.Printf("%v: %p unlease %v %v %v\n", db.GetName(), fssrv, sid, msg.Type(), req)
		if err := sess.Unlease(req.Wnames); err != nil {
			return nil, &np.Rerror{err.Error()}
		}
		return reply, nil
	default:
		// log.Printf("%v: %p %v %v\n", db.GetName(), fssrv, msg.Type(), req)
	}
	return nil, nil
}

func (fsssrv *FsServer) sendReply(t np.Ttag, reply np.Tmsg, replies chan *np.Fcall) {
	fcall := &np.Fcall{}
	fcall.Type = reply.Type()
	fcall.Msg = reply
	fcall.Tag = t
	replies <- fcall
}

// Serialize thread that serve a request for the same session.
// Threads may block in sesscond.Wait() and give up sess lock
// temporarily.  XXX doesn't guarantee the order in which received
func (fssrv *FsServer) serve(sess *session.Session, fc *np.Fcall, replies chan *np.Fcall) {
	sess.Lock()
	sess.Inc()
	reply, rerror := sess.Dispatch(fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fssrv.sendReply(fc.Tag, reply, replies)
	sess.Dec()
	sess.Unlock()
}

func (fssrv *FsServer) CloseSession(sid np.Tsession, replies chan *np.Fcall) {
	sess, ok := fssrv.st.Lookup(sid)
	if !ok {
		log.Fatalf("CloseSession unknown session %v\n", sid)
	}

	// Several threads maybe waiting in a sesscond. DeleteSess
	// will unblock them so that they can bail out.
	fssrv.sct.DeleteSess(sid)

	log.Printf("%v: CloseSession: wait threads b %v t %v\n", sid, sess.Nblocked, sess.Nthread)

	// Wait until nthread == 0
	sess.Lock()
	sess.WaitNthreadZero()
	sess.Unlock()

	// Detach the session to remove ephemeral files and close open fids.
	fssrv.st.Detach(sid)

	// close the reply channel, so that conn writer() terminates
	close(replies)
}
