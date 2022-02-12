package fssrv

import (
	"log"
	"runtime/debug"

	"ulambda/ctx"
	"ulambda/fences"
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
	"ulambda/threadmgr"
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
	tm    *threadmgr.ThreadMgrTable
	wt    *watch.WatchTable
	rft   *fences.RecentTable
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
	fssrv.rft = fences.MakeRecentTable()
	fssrv.tm = threadmgr.MakeThreadMgrTable(fssrv.process)
	fssrv.st = session.MakeSessionTable(mkps, fssrv, fssrv.rft, fssrv.tm)
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

func (fssrv *FsServer) GetRecentFences() *fences.RecentTable {
	return fssrv.rft
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
			log.Printf("%v: Error Started: %v", proc.GetProgram(), err)
		}
		if err := fssrv.pclnt.WaitEvict(proc.GetPid()); err != nil {
			debug.PrintStack()
			log.Printf("%v: Error WaitEvict: %v", proc.GetProgram(), err)
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
	// New thread about to start
	sess.IncThreads()
	sess.GetThread().Process(fc, replies)
}

func (fssrv *FsServer) process(fc *np.Fcall, replies chan *np.Fcall) {
	sess := fssrv.st.Alloc(fc.Session)
	fssrv.stats.StatInfo().Inc(fc.Msg.Type())
	fssrv.serve(sess, fc, replies)
}

// Register and unregister fences, and check fresness of fences before
// processing a request.
func (fssrv *FsServer) fenceSession(sess *session.Session, msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	switch req := msg.(type) {
	case np.Tcreate, np.Tread, np.Twrite, np.Tremove, np.Tremovefile, np.Tstat, np.Twstat, np.Trenameat, np.Tgetfile, np.Tsetfile:
		// Check that all fences that this session registered
		// are recent.  Another session may have registered a
		// more recent one in the recent fences table
		err := sess.CheckFences(fssrv.fsl)
		if err != nil {
			return nil, err.Rerror()
		}
		// log.Printf("%v: %v %v %v\n", proc.GetProgram(), sess.Sid, msg.Type(), req)
	case np.Tregfence:
		log.Printf("%p: Fence %v %v\n", fssrv, sess.Sid, req)
		err := fssrv.rft.UpdateFence(req.Fence)
		if err != nil {
			log.Printf("%v: Fence %v %v err %v\n", proc.GetProgram(), sess.Sid, req, err)
			return nil, err.Rerror()
		}
		// Fence was present in recent fences table and not
		// stale, or was not present. Now mark that all ops on
		// this sess must be checked against the most
		// recently-seen fence in rft.  Another sess may
		// register a more recent fence in rft in the future,
		// and then ops on this session should fail.  Fence
		// may be called many times on sess, because client
		// may register a more recent fence.
		sess.Fence(req)
		reply := &np.Ropen{}
		return reply, nil
	case np.Tunfence:
		log.Printf("%p: Unfence %v %v\n", fssrv, sess.Sid, req)
		err := sess.Unfence(req.Fence.FenceId)
		if err != nil {
			return nil, err.Rerror()
		}
		reply := &np.Ropen{}
		return reply, nil
	case np.Trmfence:
		log.Printf("%p: Rmfence %v %v\n", fssrv, sess.Sid, req)
		err := fssrv.rft.RmFence(req.Fence)
		if err != nil {
			return nil, err.Rerror()
		}
		reply := &np.Ropen{}
		return reply, nil
	default: // Tversion, Tauth, Tflush, Twalk, Tclunk, Topen, Tmkfence
		// log.Printf("%v: %v %v %v\n", proc.GetProgram(), sess.Sid, msg.Type(), req)
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
	defer sess.DecThreads()
	reply, rerror := sess.Dispatch(fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fssrv.sendReply(fc.Tag, reply, replies)
}

func (fssrv *FsServer) CloseSession(sid np.Tsession, replies chan *np.Fcall) {
	sess, ok := fssrv.st.Lookup(sid)
	if !ok {
		// client start TCP connection, but then failed before sending
		// any messages.
		close(replies)
		return
	}

	// XXX remove fence from sess, so that fence maybe free from seen table

	// Several threads maybe waiting in a sesscond. DeleteSess
	// will unblock them so that they can bail out.
	fssrv.sct.DeleteSess(sid)

	// Wait until nthread == 0
	sess.WaitThreads()

	// Stop sess thread.
	fssrv.st.KillSessThread(sid)

	// Detach the session to remove ephemeral files and close open fids.
	fssrv.st.Detach(sid)

	// close the reply channel, so that conn writer() terminates
	close(replies)
}
