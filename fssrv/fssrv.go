package fssrv

import (
	"log"
	"runtime/debug"

	"ulambda/ctx"
	"ulambda/dir"
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
	"ulambda/snapshot"
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
	addr    string
	root    fs.Dir
	mkps    protsrv.MkProtServer
	rps     protsrv.RestoreProtServer
	stats   *stats.Stats
	st      *session.SessionTable
	sct     *sesscond.SessCondTable
	tm      *threadmgr.ThreadMgrTable
	wt      *watch.WatchTable
	rft     *fences.RecentTable
	srv     *netsrv.NetServer
	replSrv repl.Server
	rc      *repl.ReplyCache
	pclnt   *procclnt.ProcClnt
	done    bool
	ch      chan bool
	fsl     *fslib.FsLib
}

func MakeFsServer(root fs.Dir, addr string, fsl *fslib.FsLib,
	mkps protsrv.MkProtServer, rps protsrv.RestoreProtServer, pclnt *procclnt.ProcClnt,
	config repl.Config) *FsServer {
	fssrv := &FsServer{}
	fssrv.root = root
	fssrv.addr = addr
	fssrv.mkps = mkps
	fssrv.stats = stats.MkStats(fssrv.root)
	fssrv.rft = fences.MakeRecentTable()
	fssrv.tm = threadmgr.MakeThreadMgrTable(fssrv.process, config != nil)
	fssrv.st = session.MakeSessionTable(mkps, fssrv, fssrv.rft, fssrv.tm)
	fssrv.sct = sesscond.MakeSessCondTable(fssrv.st)
	fssrv.wt = watch.MkWatchTable(fssrv.sct)
	fssrv.srv = netsrv.MakeNetServer(fssrv, addr)
	if config == nil {
		fssrv.replSrv = nil
	} else {
		fssrv.rc = repl.MakeReplyCache()
		fssrv.replSrv = config.MakeServer(fssrv.tm.AddThread())
		fssrv.replSrv.Start()
		log.Printf("Starting repl server")
	}
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

func (fssrv *FsServer) Snapshot() []byte {
	if fssrv.replSrv == nil {
		log.Fatalf("FATAL: Tried to snapshot an unreplicated server %v", proc.GetName())
	}
	snap := snapshot.MakeSnapshot()
	return snap.Snapshot(fssrv.root.(*dir.DirImpl), fssrv.st, fssrv.tm, fssrv.rft, fssrv.rc)
}

func (fssrv *FsServer) Sess(sid np.Tsession) *session.Session {
	sess, ok := fssrv.st.Lookup(sid)
	if !ok {
		log.Fatalf("FATAL %v: no sess %v\n", proc.GetName(), sid)
		return nil
	}
	return sess
}

// The server using fssrv is ready to take requests. Keep serving
// until fssrv is told to stop using Done().
func (fssrv *FsServer) Serve() {
	// Non-intial-named services wait on the pclnt infrastructure. Initial named waits on the channel.
	if fssrv.pclnt != nil {
		if err := fssrv.pclnt.Started(proc.GetPid()); err != nil {
			debug.PrintStack()
			log.Printf("%v: Error Started: %v", proc.GetName(), err)
		}
		if err := fssrv.pclnt.WaitEvict(proc.GetPid()); err != nil {
			debug.PrintStack()
			log.Printf("%v: Error WaitEvict: %v", proc.GetName(), err)
		}
	} else {
		<-fssrv.ch
	}
}

// The server using fssrv is done; exit.
func (fssrv *FsServer) Done() {
	if fssrv.pclnt != nil {
		fssrv.pclnt.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusEvicted))
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
	// New thread about to start
	sess.IncThreads()
	if fssrv.replSrv == nil {
		sess.GetThread().Process(fc, replies)
	} else {
		fssrv.replSrv.Process(fc, replies)
	}
}

func (fssrv *FsServer) sendReply(request *np.Fcall, reply np.Tmsg, replies chan *np.Fcall) {
	// Store the reply in the reply cache if this is a replicated server.
	if fssrv.replSrv != nil {
		fssrv.rc.Put(request, reply)
	}
	// The replies channel may be nil if this is a replicated op which came
	// through raft. In this case, a reply is not needed.
	if replies != nil {
		fcall := &np.Fcall{}
		fcall.Type = reply.Type()
		fcall.Msg = reply
		fcall.Tag = request.Tag
		replies <- fcall
	}
}

func (fssrv *FsServer) process(fc *np.Fcall, replies chan *np.Fcall) {
	if fssrv.replSrv != nil {
		// Reply cache needs to live under the replication layer in order to
		// handle duplicate requests. These may occur if, for example:
		//
		// 1. A client connects to replica A and issues a request.
		// 2. Replica A pushes the request through raft.
		// 3. Before responding to the client, replica A crashes.
		// 4. The client connects to replica B, and retries the request *before*
		//    replica B hears about the request through raft.
		// 5. Replica B pushes the request through raft.
		// 6. Replica B now receives the same request twice through raft's apply
		//    channel, and will try to execute the request twice.
		//
		// In order to handle this, we can use the reply cache to deduplicate
		// requests. Since requests execute sequentially, one of the requests will
		// register itself first in the reply cache. The other request then just
		// has to wait on the reply future in order to send the reply. This can
		// happen asynchronously since it doesn't affect server state, and the
		// asynchrony is necessary in order to allow other ops on the thread to
		// make progress. We coulld optionally use sessconds, but they're kind of
		// overkill since we don't care about ordering in this case.
		if replyFuture, ok := fssrv.rc.Get(fc); ok {
			go func() {
				fssrv.sendReply(fc, replyFuture.Await(), replies)
			}()
			return
		}
		// If this request has not been registered with the reply cache yet, register
		// it.
		fssrv.rc.Register(fc)
	}
	sess := fssrv.st.Alloc(fc.Session)
	fssrv.stats.StatInfo().Inc(fc.Msg.Type())
	fssrv.serve(sess, fc, replies)
}

func (fssrv *FsServer) serve(sess *session.Session, fc *np.Fcall, replies chan *np.Fcall) {
	reply, rerror := sess.Dispatch(fc.Msg)
	// We decrement the number of waiting threads if this request was made to
	// this server (it didn't come through raft), which will only be the case
	// when replies is not nil or the fcall is of type Detach (in which case
	// replies will be nil, since detaches are generated at the server).
	if replies != nil || fc.GetMsg().Type() == np.TTdetach {
		defer sess.DecThreads()
	}
	if rerror != nil {
		reply = *rerror
	}
	fssrv.sendReply(fc, reply, replies)
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

	// Wait until nthread == 0. Detach is guaranteed to have been processed since
	// it was enqueued by the reader function before calling CloseSession
	// (incrementing nthread). We need to process Detaches (and sess cond closes)
	// through the session thread manager since they generate wakeups and need to
	// be properly serialized (especially for replication).
	sess.WaitThreads()

	// Stop sess thread.
	fssrv.st.KillSessThread(sid)

	// close the reply channel, so that conn writer() terminates
	close(replies)
}
