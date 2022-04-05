package sesssrv

import (
	"log"
	"reflect"
	"runtime/debug"

	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fencefs"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/netsrv"
	np "ulambda/ninep"
	"ulambda/overlay"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/repl"
	"ulambda/sesscond"
	"ulambda/session"
	"ulambda/snapshot"
	"ulambda/stats"
	"ulambda/threadmgr"
	"ulambda/watch"
)

//
// There is one SessSrv per server. The SessSrv has one protsrv per
// session (i.e., TCP connection). Each session may multiplex several
// users.
//
// SessSrv has a table with all sess conds in use so that it can
// unblock threads that are waiting in a sess cond when a session
// closes.
//

type SessSrv struct {
	addr       string
	root       fs.Dir
	mkps       np.MkProtServer
	rps        np.RestoreProtServer
	stats      *stats.Stats
	st         *session.SessionTable
	sm         *session.SessionMgr
	sct        *sesscond.SessCondTable
	tmt        *threadmgr.ThreadMgrTable
	wt         *watch.WatchTable
	ffs        fs.Dir
	srv        *netsrv.NetServer
	replSrv    repl.Server
	rc         *repl.ReplyCache
	pclnt      *procclnt.ProcClnt
	snap       *snapshot.Snapshot
	done       bool
	replicated bool
	ch         chan bool
	fsl        *fslib.FsLib
}

func MakeSessSrv(root fs.Dir, addr string, fsl *fslib.FsLib,
	mkps np.MkProtServer, rps np.RestoreProtServer, pclnt *procclnt.ProcClnt,
	config repl.Config) *SessSrv {
	ssrv := &SessSrv{}
	ssrv.replicated = config != nil && !reflect.ValueOf(config).IsNil()
	dirover := overlay.MkDirOverlay(root)
	ssrv.root = dirover
	ssrv.addr = addr
	ssrv.mkps = mkps
	ssrv.rps = rps
	ssrv.stats = stats.MkStatsDev(ssrv.root)
	ssrv.tmt = threadmgr.MakeThreadMgrTable(ssrv.srvfcall, ssrv.replicated)
	ssrv.st = session.MakeSessionTable(mkps, ssrv, ssrv.tmt)
	ssrv.sm = session.MakeSessionMgr(ssrv.st, ssrv.SrvFcall)
	ssrv.sct = sesscond.MakeSessCondTable(ssrv.st)
	ssrv.wt = watch.MkWatchTable(ssrv.sct)
	ssrv.srv = netsrv.MakeNetServer(ssrv, addr)
	ssrv.rc = repl.MakeReplyCache()
	ssrv.ffs = fencefs.MakeRoot(ctx.MkCtx("", 0, nil))

	dirover.Mount(np.STATSD, ssrv.stats)
	dirover.Mount(np.FENCEDIR, ssrv.ffs.(*dir.DirImpl))

	if !ssrv.replicated {
		ssrv.replSrv = nil
	} else {
		snapDev := snapshot.MakeDev(ssrv, nil, ssrv.root)
		dirover.Mount(np.SNAPDEV, snapDev)

		ssrv.replSrv = config.MakeServer(ssrv.tmt.AddThread())
		ssrv.replSrv.Start()
		log.Printf("Starting repl server")
	}
	ssrv.pclnt = pclnt
	ssrv.ch = make(chan bool)
	ssrv.fsl = fsl
	ssrv.stats.MonitorCPUUtil()
	return ssrv
}

func (ssrv *SessSrv) SetFsl(fsl *fslib.FsLib) {
	ssrv.fsl = fsl
}

func (ssrv *SessSrv) GetSessCondTable() *sesscond.SessCondTable {
	return ssrv.sct
}

func (ssrv *SessSrv) Root() fs.Dir {
	return ssrv.root
}

func (ssrv *SessSrv) Snapshot() []byte {
	log.Printf("Snapshot %v", proc.GetPid())
	if !ssrv.replicated {
		db.DFatalf("Tried to snapshot an unreplicated server %v", proc.GetName())
	}
	ssrv.snap = snapshot.MakeSnapshot(ssrv)
	return ssrv.snap.Snapshot(ssrv.root.(*overlay.DirOverlay), ssrv.st, ssrv.tmt, ssrv.rc)
}

func (ssrv *SessSrv) Restore(b []byte) {
	if !ssrv.replicated {
		db.DFatalf("Tried to restore an unreplicated server %v", proc.GetName())
	}
	// Store snapshot for later use during restore.
	ssrv.snap = snapshot.MakeSnapshot(ssrv)
	ssrv.stats.Done()
	// XXX How do we install the sct and wt? How do we sunset old state when
	// installing a snapshot on a running server?
	ssrv.root, ssrv.ffs, ssrv.stats, ssrv.st, ssrv.tmt, ssrv.rc = ssrv.snap.Restore(ssrv.mkps, ssrv.rps, ssrv, ssrv.tmt.AddThread(), ssrv.srvfcall, ssrv.st, ssrv.rc, b)
	ssrv.stats.MonitorCPUUtil()
	ssrv.sct.St = ssrv.st
	ssrv.sm.Stop()
	ssrv.sm = session.MakeSessionMgr(ssrv.st, ssrv.SrvFcall)
}

func (ssrv *SessSrv) Sess(sid np.Tsession) *session.Session {
	sess, ok := ssrv.st.Lookup(sid)
	if !ok {
		db.DFatalf("%v: no sess %v\n", proc.GetName(), sid)
		return nil
	}
	return sess
}

// The server using ssrv is ready to take requests. Keep serving
// until ssrv is told to stop using Done().
func (ssrv *SessSrv) Serve() {
	// Non-intial-named services wait on the pclnt infrastructure. Initial named waits on the channel.
	if ssrv.pclnt != nil {
		if err := ssrv.pclnt.Started(); err != nil {
			debug.PrintStack()
			log.Printf("%v: Error Started: %v", proc.GetName(), err)
		}
		if err := ssrv.pclnt.WaitEvict(proc.GetPid()); err != nil {
			log.Printf("%v: Error WaitEvict: %v", proc.GetName(), err)
		}
	} else {
		<-ssrv.ch
	}
	log.Printf("done serving\n")
	ssrv.st.WaitClosed()
}

// The server using ssrv is done; exit.
func (ssrv *SessSrv) Done() {
	if ssrv.pclnt != nil {
		ssrv.pclnt.Exited(proc.MakeStatus(proc.StatusEvicted))
	} else {
		if !ssrv.done {
			ssrv.done = true
			ssrv.ch <- true

		}
	}
	ssrv.stats.Done()
}

func (ssrv *SessSrv) MyAddr() string {
	return ssrv.srv.MyAddr()
}

func (ssrv *SessSrv) GetStats() *stats.Stats {
	return ssrv.stats
}

func (ssrv *SessSrv) GetWatchTable() *watch.WatchTable {
	return ssrv.wt
}

func (ssrv *SessSrv) GetSnapshotter() *snapshot.Snapshot {
	return ssrv.snap
}

func (ssrv *SessSrv) AttachTree(uname string, aname string, sessid np.Tsession) (fs.Dir, fs.CtxI) {
	return ssrv.root, ctx.MkCtx(uname, sessid, ssrv.sct)
}

// New session or new connection for existing session
func (ssrv *SessSrv) Register(sid np.Tsession, conn *np.Conn) *np.Err {
	db.DPrintf("SESSSRV", "Register sid %v %v\n", sid, conn)
	sess := ssrv.st.Alloc(sid)
	return sess.SetConn(conn)
}

func (ssrv *SessSrv) SrvFcall(fc *np.Fcall) {
	sess, ok := ssrv.st.Lookup(fc.Session)
	if !ok {
		db.DFatalf("SrvFcall: no session %v\n", fc.Session)
	}
	if !ssrv.replicated {
		sess.GetThread().Process(fc)
	} else {
		ssrv.replSrv.Process(fc)
	}
}

func (ssrv *SessSrv) sendReply(request *np.Fcall, reply np.Tmsg, sess *session.Session) {
	fcall := np.MakeFcallReply(request, reply)

	db.DPrintf("SESSSRV", "Request %v start sendReply %v", request, fcall)

	// Store the reply in the reply cache.
	ssrv.rc.Put(request, fcall)

	// If a client sent the request (seqno != 0) (as opposed to an
	// internally-generated detach), send reply.
	if request.Seqno != 0 {
		sess.SendConn(fcall)
	}
	db.DPrintf("SESSSRV", "Request %v done sendReply %v", request, fcall)
}

func (ssrv *SessSrv) srvfcall(fc *np.Fcall) {
	// If this is a replicated op received through raft (not
	// directly from a client), the first time Alloc is called
	// will be in this function, so the conn will be set to
	// nil. If it came from the client, the conn will already be
	// set.
	sess := ssrv.st.Alloc(fc.Session)
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
	if replyFuture, ok := ssrv.rc.Get(fc); ok {
		db.DPrintf("SESSSRV", "Request %v reply in cache", fc)
		go func() {
			ssrv.sendReply(fc, replyFuture.Await().GetMsg(), sess)
		}()
		return
	}
	db.DPrintf("SESSSRV", "Request %v reply not in cache", fc)
	// If this request has not been registered with the reply cache yet, register
	// it.
	ssrv.rc.Register(fc)
	ssrv.stats.StatInfo().Inc(fc.Msg.Type())
	ssrv.fenceFcall(sess, fc)
}

// Fence an fcall, if the call has a fence associated with it.  Note: don't fence blocking
// ops.
func (ssrv *SessSrv) fenceFcall(sess *session.Session, fc *np.Fcall) {
	db.DPrintf("FENCES", "fenceFcall %v fence %v\n", fc.Type, fc.Fence)
	if f, err := fencefs.CheckFence(ssrv.ffs, fc.Fence); err != nil {
		reply := *err.Rerror()
		ssrv.sendReply(fc, reply, sess)
		return
	} else {
		if f == nil {
			ssrv.serve(sess, fc)
		} else {
			defer f.Unlock()
			ssrv.serve(sess, fc)
		}
	}
}

func (ssrv *SessSrv) serve(sess *session.Session, fc *np.Fcall) {
	db.DPrintf("SESSSRV", "Dispatch request %v", fc)
	reply, close, rerror := sess.Dispatch(fc.Msg)
	db.DPrintf("SESSSRV", "Done dispatch request %v close? %v", fc, close)

	if rerror != nil {
		reply = *rerror
	}

	ssrv.sendReply(fc, reply, sess)

	if close {
		// Dispatch() signaled to close the session.
		sess.Close()
	}
}

func (ssrv *SessSrv) PartitionClient(permanent bool) {
	if permanent {
		ssrv.sm.TimeoutSession()
	} else {
		ssrv.sm.CloseConn()
	}
}
