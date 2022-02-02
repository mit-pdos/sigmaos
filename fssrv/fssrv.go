package fssrv

import (
	"log"
	"runtime/debug"

	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/fence"
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
	addr       string
	root       fs.Dir
	mkps       protsrv.MkProtServer
	stats      *stats.Stats
	st         *session.SessionTable
	sct        *sesscond.SessCondTable
	wt         *watch.WatchTable
	seenFences *fence.FenceTable
	srv        *netsrv.NetServer
	pclnt      *procclnt.ProcClnt
	done       bool
	ch         chan bool
	fsl        *fslib.FsLib
}

func MakeFsServer(root fs.Dir, addr string, fsl *fslib.FsLib,
	mkps protsrv.MkProtServer, pclnt *procclnt.ProcClnt,
	config repl.Config) *FsServer {
	fssrv := &FsServer{}
	fssrv.root = root
	fssrv.addr = addr
	fssrv.mkps = mkps
	fssrv.stats = stats.MkStats(fssrv.root)
	fssrv.seenFences = fence.MakeFenceTable()
	fssrv.st = session.MakeSessionTable(mkps, fssrv, fssrv.seenFences)
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

func (fssrv *FsServer) GetSeenFences() *fence.FenceTable {
	return fssrv.seenFences
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
			log.Printf("%v: Error Started: %v", db.GetName(), err)
		}
		if err := fssrv.pclnt.WaitEvict(proc.GetPid()); err != nil {
			debug.PrintStack()
			log.Printf("%v: Error WaitEvict: %v", db.GetName(), err)
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
	// New thread about to start
	sess.IncThreads()
	go fssrv.serve(sess, fc, replies)
}

// Register and unregister fences, and check fresness of fences before
// processing a request.
func (fssrv *FsServer) fenceSession(sess *session.Session, msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	switch req := msg.(type) {
	case np.Tcreate, np.Tread, np.Twrite, np.Tremove, np.Tremovefile, np.Tstat, np.Twstat, np.Trenameat, np.Tgetfile, np.Tsetfile:
		// Check that all fences that this session registered
		// are recent.  Another session may have registered a
		// more recent one in seenFences.
		err := sess.CheckFences(fssrv.fsl)
		if err != nil {
			return nil, &np.Rerror{err.Error()}
		}
	case np.Tregfence:
		log.Printf("%p: Fence %v %v\n", fssrv, sess.Sid, req)
		err := fssrv.seenFences.Register(req)
		if err != nil {
			log.Printf("%v: Fence %v %v err %v\n", db.GetName(), sess.Sid, req, err)
			return nil, &np.Rerror{err.Error()}
		}
		// Fence was present in seenFences and not stale, or
		// was not present. Now mark that all ops on this sess
		// must be checked against the most recently-seen
		// fence in seenFences.  Another sess may register a
		// more recent fence in seenFences in the future, and
		// then ops on this session should fail.
		err = sess.Fence(req)
		if err != nil {
			log.Printf("%v: Fence sess %v %v err %v\n", db.GetName(), sess.Sid, req, err)
			return nil, &np.Rerror{err.Error()}
		}
		reply := &np.Ropen{}
		return reply, nil
	case np.Tunfence:
		log.Printf("%p: Unfence %v %v\n", fssrv, sess.Sid, req)
		err := fssrv.seenFences.Unregister(req.Fence)
		if err != nil {
			return nil, &np.Rerror{err.Error()}
		}
		err = sess.Unfence(req.Fence.FenceId)
		if err != nil {
			return nil, &np.Rerror{err.Error()}
		}
		reply := &np.Ropen{}
		return reply, nil
	default: // Tversion, Tauth, Tflush, Twalk, Tclunk, Topen, Tmkfence
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
	defer sess.DecThreads()
	sess.Lock()
	reply, rerror := sess.Dispatch(fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fssrv.sendReply(fc.Tag, reply, replies)
	sess.Unlock()
}

func (fssrv *FsServer) CloseSession(sid np.Tsession, replies chan *np.Fcall) {
	sess, ok := fssrv.st.Lookup(sid)
	if !ok {
		// client start TCP connection, but then failed before sending
		// any messages.
		log.Printf("Warning: CloseSession unknown session %v\n", sid)
		close(replies)
		return
	}

	// XXX remove fence from sess, so that fence maybe free from seen table

	// Several threads maybe waiting in a sesscond. DeleteSess
	// will unblock them so that they can bail out.
	fssrv.sct.DeleteSess(sid)

	// Wait until nthread == 0
	sess.WaitThreads()

	// Detach the session to remove ephemeral files and close open fids.
	fssrv.st.Detach(sid)

	// close the reply channel, so that conn writer() terminates
	close(replies)
}
