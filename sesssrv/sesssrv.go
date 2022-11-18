package sesssrv

import (
	"reflect"
	"runtime/debug"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fencefs"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/kernel"
	"sigmaos/lockmap"
	"sigmaos/netsrv"
	np "sigmaos/ninep"
	"sigmaos/overlay"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/repl"
	"sigmaos/sesscond"
	"sigmaos/sessstatesrv"
	"sigmaos/snapshot"
	"sigmaos/stats"
	"sigmaos/threadmgr"
	"sigmaos/version"
	"sigmaos/watch"
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
	st         *sessstatesrv.SessionTable
	sm         *sessstatesrv.SessionMgr
	sct        *sesscond.SessCondTable
	tmt        *threadmgr.ThreadMgrTable
	plt        *lockmap.PathLockTable
	wt         *watch.WatchTable
	vt         *version.VersionTable
	ffs        fs.Dir
	srv        *netsrv.NetServer
	replSrv    repl.Server
	pclnt      *procclnt.ProcClnt
	snap       *snapshot.Snapshot
	done       bool
	replicated bool
	ch         chan bool
	fsl        *fslib.FsLib
	detach     fs.DetachF
}

func MakeSessSrv(root fs.Dir, addr string, fsl *fslib.FsLib,
	mkps np.MkProtServer, rps np.RestoreProtServer, pclnt *procclnt.ProcClnt,
	config repl.Config, detach fs.DetachF) *SessSrv {
	ssrv := &SessSrv{}
	ssrv.replicated = config != nil && !reflect.ValueOf(config).IsNil()
	dirover := overlay.MkDirOverlay(root)
	ssrv.root = dirover
	ssrv.addr = addr
	ssrv.mkps = mkps
	ssrv.rps = rps
	ssrv.stats = stats.MkStatsDev(ssrv.root)
	ssrv.tmt = threadmgr.MakeThreadMgrTable(ssrv.srvfcall, ssrv.replicated)
	ssrv.st = sessstatesrv.MakeSessionTable(mkps, ssrv, ssrv.tmt)
	ssrv.sct = sesscond.MakeSessCondTable(ssrv.st)
	ssrv.plt = lockmap.MkPathLockTable()
	ssrv.wt = watch.MkWatchTable(ssrv.sct)
	ssrv.vt = version.MkVersionTable()
	ssrv.vt.Insert(ssrv.root.Path())

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
		db.DPrintf(db.ALWAYS, "Starting repl server: %v", config)
	}
	ssrv.srv = netsrv.MakeNetServer(ssrv, addr)
	ssrv.sm = sessstatesrv.MakeSessionMgr(ssrv.st, ssrv.SrvFcall)
	db.DPrintf("SESSSRV0", "Listen on address: %v", ssrv.srv.MyAddr())
	ssrv.pclnt = pclnt
	ssrv.ch = make(chan bool)
	ssrv.fsl = fsl
	ssrv.detach = detach
	return ssrv
}

func (ssrv *SessSrv) SetFsl(fsl *fslib.FsLib) {
	ssrv.fsl = fsl
}

func (ssrv *SessSrv) GetSessCondTable() *sesscond.SessCondTable {
	return ssrv.sct
}

func (ssrv *SessSrv) GetPathLockTable() *lockmap.PathLockTable {
	return ssrv.plt
}

func (ssrv *SessSrv) Root() fs.Dir {
	return ssrv.root
}

func (ssrv *SessSrv) GetDetach() fs.DetachF {
	return ssrv.detach
}

func (ssrv *SessSrv) Snapshot() []byte {
	db.DPrintf(db.ALWAYS, "Snapshot %v", proc.GetPid())
	if !ssrv.replicated {
		db.DFatalf("Tried to snapshot an unreplicated server %v", proc.GetName())
	}
	ssrv.snap = snapshot.MakeSnapshot(ssrv)
	return ssrv.snap.Snapshot(ssrv.root.(*overlay.DirOverlay), ssrv.st, ssrv.tmt)
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
	ssrv.root, ssrv.ffs, ssrv.stats, ssrv.st, ssrv.tmt = ssrv.snap.Restore(ssrv.mkps, ssrv.rps, ssrv, ssrv.tmt.AddThread(), ssrv.srvfcall, ssrv.st, b)
	ssrv.sct.St = ssrv.st
	ssrv.sm.Stop()
	ssrv.sm = sessstatesrv.MakeSessionMgr(ssrv.st, ssrv.SrvFcall)
}

func (ssrv *SessSrv) Sess(sid np.Tsession) *sessstatesrv.Session {
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
		// If this is a kernel proc, register the subsystem info for the realmmgr
		if proc.GetIsPrivilegedProc() {
			si := kernel.MakeSubsystemInfo(proc.GetPid(), ssrv.MyAddr(), proc.GetNodedId())
			kernel.RegisterSubsystemInfo(ssrv.fsl, si)
		}
		if err := ssrv.pclnt.Started(); err != nil {
			debug.PrintStack()
			db.DPrintf(db.ALWAYS, "Error Started: %v", err)
		}
		if err := ssrv.pclnt.WaitEvict(proc.GetPid()); err != nil {
			db.DPrintf(db.ALWAYS, "Error WaitEvict: %v", err)
		}
	} else {
		<-ssrv.ch
	}
	db.DPrintf("SESSSRV", "Done serving")
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

func (ssrv *SessSrv) QueueLen() int {
	return ssrv.st.QueueLen()
}

func (ssrv *SessSrv) GetWatchTable() *watch.WatchTable {
	return ssrv.wt
}

func (ssrv *SessSrv) GetVersionTable() *version.VersionTable {
	return ssrv.vt
}

func (ssrv *SessSrv) GetSnapshotter() *snapshot.Snapshot {
	return ssrv.snap
}

func (ssrv *SessSrv) AttachTree(uname string, aname string, sessid np.Tsession) (fs.Dir, fs.CtxI) {
	return ssrv.root, ctx.MkCtx(uname, sessid, ssrv.sct)
}

// New session or new connection for existing session
func (ssrv *SessSrv) Register(cid np.Tclient, sid np.Tsession, conn np.Conn) *np.Err {
	db.DPrintf("SESSSRV", "Register sid %v %v\n", sid, conn)
	sess := ssrv.st.Alloc(cid, sid)
	return sess.SetConn(conn)
}

// Disassociate a connection with a session, and let it close gracefully.
func (ssrv *SessSrv) Unregister(cid np.Tclient, sid np.Tsession, conn np.Conn) {
	// If this connection hasn't been associated with a session yet, return.
	if sid == np.NoSession {
		return
	}
	sess := ssrv.st.Alloc(cid, sid)
	sess.UnsetConn(conn)
}

func (ssrv *SessSrv) SrvFcall(fc *np.Fcall) {
	sess, ok := ssrv.st.Lookup(fc.Session)
	// Server-generated heartbeats will have session number 0. Pass them through.
	if !ok && fc.Session != 0 {
		db.DFatalf("SrvFcall: no session %v for req %v\n", fc.Session, fc)
	}
	if !ssrv.replicated {
		// If the fcall is a server-generated heartbeat, don't worry about
		// processing it sequentially on the session's thread.
		if fc.Session == 0 {
			ssrv.srvfcall(fc)
		} else {
			sess.GetThread().Process(fc)
		}
	} else {
		ssrv.replSrv.Process(fc)
	}
}

func (ssrv *SessSrv) sendReply(request *np.Fcall, reply np.Tmsg, sess *sessstatesrv.Session) {
	fcall := np.MakeFcallReply(request, reply)

	// Store the reply in the reply cache.
	ok := sess.GetReplyTable().Put(request, fcall)

	db.DPrintf("SESSSRV", "sendReply req %v rep %v ok %v", request, fcall, ok)

	// If a client sent the request (seqno != 0) (as opposed to an
	// internally-generated detach or heartbeat), send reply.
	if request.Seqno != 0 && ok {
		sess.SendConn(fcall)
	}
}

func (ssrv *SessSrv) srvfcall(fc *np.Fcall) {
	// If this was a server-generated heartbeat message, heartbeat all of the
	// contained sessions, and then return immediately (no further processing is
	// necessary).
	if fc.Session == 0 {
		ssrv.st.ProcessHeartbeats(fc.Msg.(*np.Theartbeat))
		return
	}
	// If this is a replicated op received through raft (not
	// directly from a client), the first time Alloc is called
	// will be in this function, so the conn will be set to
	// nil. If it came from the client, the conn will already be
	// set.
	sess := ssrv.st.Alloc(fc.Client, fc.Session)
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
	if replyFuture, ok := sess.GetReplyTable().Get(fc); ok {
		db.DPrintf("SESSSRV", "srvfcall %v reply in cache", fc)
		go func() {
			ssrv.sendReply(fc, replyFuture.Await().GetMsg(), sess)
		}()
		return
	}
	db.DPrintf("SESSSRV", "srvfcall %v reply not in cache", fc)
	if ok := sess.GetReplyTable().Register(fc); ok {
		db.DPrintf("RTABLE", "table: %v", sess.GetReplyTable())
		ssrv.stats.StatInfo().Inc(fc.Msg.Type())
		ssrv.fenceFcall(sess, fc)
	}
}

// Fence an fcall, if the call has a fence associated with it.  Note: don't fence blocking
// ops.
func (ssrv *SessSrv) fenceFcall(sess *sessstatesrv.Session, fc *np.Fcall) {
	db.DPrintf("FENCES", "fenceFcall %v fence %v\n", fc.Type, fc.Fence)
	if f, err := fencefs.CheckFence(ssrv.ffs, fc.Fence); err != nil {
		reply := err.Rerror()
		ssrv.sendReply(fc, reply, sess)
		return
	} else {
		if f == nil {
			ssrv.serve(sess, fc)
		} else {
			defer f.RUnlock()
			ssrv.serve(sess, fc)
		}
	}
}

func (ssrv *SessSrv) serve(sess *sessstatesrv.Session, fc *np.Fcall) {
	db.DPrintf("SESSSRV", "Dispatch request %v", fc)
	reply, close, rerror := sess.Dispatch(fc.Msg)
	db.DPrintf("SESSSRV", "Done dispatch request %v close? %v", fc, close)

	if rerror != nil {
		reply = rerror
	}

	ssrv.sendReply(fc, reply, sess)

	if close {
		// Dispatch() signaled to close the sessstatesrv.
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
