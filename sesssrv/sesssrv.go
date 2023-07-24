package sesssrv

import (
	"reflect"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/ephemeralmap"
	"sigmaos/fencefs"
	"sigmaos/fs"
	"sigmaos/lockmap"
	"sigmaos/netsrv"
	"sigmaos/overlaydir"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/serr"
	"sigmaos/sesscond"
	"sigmaos/sessp"
	"sigmaos/sessstatesrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
	"sigmaos/snapshot"
	"sigmaos/spcodec"
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
	dirunder   fs.Dir
	dirover    *overlay.DirOverlay
	mkps       sps.MkProtServer
	rps        sps.RestoreProtServer
	stats      *stats.StatInfo
	st         *sessstatesrv.SessionTable
	sm         *sessstatesrv.SessionMgr
	sct        *sesscond.SessCondTable
	tmt        *threadmgr.ThreadMgrTable
	plt        *lockmap.PathLockTable
	wt         *watch.WatchTable
	vt         *version.VersionTable
	et         *ephemeralmap.EphemeralMap
	ffs        fs.Dir
	srv        *netsrv.NetServer
	replSrv    repl.Server
	snap       *snapshot.Snapshot
	replicated bool
	qlen       stats.Tcounter
	sc         *sigmaclnt.SigmaClnt
}

func MakeSessSrv(root fs.Dir, addr string, sc *sigmaclnt.SigmaClnt, mkps sps.MkProtServer, rps sps.RestoreProtServer, config repl.Config, attachf sps.AttachClntF, detachf sps.DetachClntF, et *ephemeralmap.EphemeralMap) *SessSrv {
	ssrv := &SessSrv{}
	ssrv.replicated = config != nil && !reflect.ValueOf(config).IsNil()
	ssrv.dirover = overlay.NewDirOverlay(root)
	ssrv.dirunder = root
	ssrv.addr = addr
	ssrv.mkps = mkps
	ssrv.rps = rps
	ssrv.et = et
	ssrv.stats = stats.MkStatsDev(ssrv.dirover)
	ssrv.tmt = threadmgr.MakeThreadMgrTable(ssrv.srvfcall, ssrv.replicated)
	ssrv.st = sessstatesrv.MakeSessionTable(mkps, ssrv, ssrv.tmt, attachf, detachf)
	ssrv.sct = sesscond.MakeSessCondTable(ssrv.st)
	ssrv.plt = lockmap.MkPathLockTable()
	ssrv.wt = watch.MkWatchTable(ssrv.sct)
	ssrv.vt = version.MkVersionTable()
	ssrv.vt.Insert(ssrv.dirover.Path())

	ssrv.ffs = fencefs.MakeRoot(ctx.MkCtxNull(), ssrv.dirover)

	ssrv.dirover.Mount(sp.STATSD, ssrv.stats)
	ssrv.dirover.Mount(sp.FENCEDIR, ssrv.ffs.(*dir.DirImpl))

	if !ssrv.replicated {
		ssrv.replSrv = nil
	} else {
		snapDev := snapshot.MakeDev(ssrv, nil, ssrv.dirover)
		ssrv.dirover.Mount(sp.SNAPDEV, snapDev)

		ssrv.replSrv = config.MakeServer(ssrv.tmt.AddThread())
		ssrv.replSrv.Start()
		db.DPrintf(db.ALWAYS, "Starting repl server: %v", config)
	}
	ssrv.srv = netsrv.MakeNetServer(ssrv, addr, spcodec.WriteFcallAndData, spcodec.ReadUnmarshalFcallAndData)
	ssrv.sm = sessstatesrv.MakeSessionMgr(ssrv.st, ssrv.SrvFcall)
	db.DPrintf(db.SESSSRV, "Listen on address: %v", ssrv.srv.MyAddr())
	ssrv.sc = sc
	return ssrv
}

func (ssrv *SessSrv) SetSigmaClnt(sc *sigmaclnt.SigmaClnt) {
	ssrv.sc = sc
}

func (ssrv *SessSrv) SigmaClnt() *sigmaclnt.SigmaClnt {
	return ssrv.sc
}

func (ssrv *SessSrv) GetSessCondTable() *sesscond.SessCondTable {
	return ssrv.sct
}

func (ssrv *SessSrv) GetPathLockTable() *lockmap.PathLockTable {
	return ssrv.plt
}

func (ssrv *SessSrv) GetEphemeralMap() *ephemeralmap.EphemeralMap {
	return ssrv.et
}

func (ssrv *SessSrv) Root(path path.Path) (fs.Dir, path.Path) {
	d := ssrv.dirunder
	if len(path) > 0 {
		o, err := ssrv.dirover.Lookup(ctx.MkCtxNull(), path[0])
		if err == nil {
			return o.(fs.Dir), path[1:]
		}
	}
	return d, path
}

func (ssrv *SessSrv) Mount(name string, dir *dir.DirImpl) {
	dir.SetParent(ssrv.dirover)
	ssrv.dirover.Mount(name, dir)
}

func (sssrv *SessSrv) RegisterDetachSess(f sps.DetachSessF, sid sessp.Tsession) *serr.Err {
	sess, ok := sssrv.st.Lookup(sid)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, sid)
	}
	sess.RegisterDetachSess(f)
	return nil
}

func (ssrv *SessSrv) Snapshot() []byte {
	db.DPrintf(db.ALWAYS, "Snapshot %v", proc.GetPid())
	if !ssrv.replicated {
		db.DFatalf("Tried to snapshot an unreplicated server %v", proc.GetName())
	}
	ssrv.snap = snapshot.MakeSnapshot(ssrv)
	return ssrv.snap.Snapshot(ssrv.dirover, ssrv.st, ssrv.tmt)
}

func (ssrv *SessSrv) Restore(b []byte) {
	if !ssrv.replicated {
		db.DFatalf("Tried to restore an unreplicated server %v", proc.GetName())
	}
	// Store snapshot for later use during restore.
	ssrv.snap = snapshot.MakeSnapshot(ssrv)
	// XXX How do we install the sct and wt? How do we sunset old
	// state when installing a snapshot on a running server?  XXX
	// dirunder should be dirover, but of type fs.Dir, but plan to
	// delete this code anyway.
	ssrv.dirunder, ssrv.ffs, ssrv.stats, ssrv.st, ssrv.tmt = ssrv.snap.Restore(ssrv.mkps, ssrv.rps, ssrv, ssrv.tmt.AddThread(), ssrv.srvfcall, ssrv.st, b)
	ssrv.sct.St = ssrv.st
	ssrv.sm.Stop()
	ssrv.sm = sessstatesrv.MakeSessionMgr(ssrv.st, ssrv.SrvFcall)
}

func (ssrv *SessSrv) Sess(sid sessp.Tsession) *sessstatesrv.Session {
	sess, ok := ssrv.st.Lookup(sid)
	if !ok {
		db.DFatalf("%v: no sess %v\n", proc.GetName(), sid)
		return nil
	}
	return sess
}

func (ssrv *SessSrv) MyAddr() string {
	return ssrv.srv.MyAddr()
}

func (ssrv *SessSrv) StopServing() error {
	if err := ssrv.srv.CloseListener(); err != nil {
		return err
	}
	if err := ssrv.st.CloseSessions(); err != nil {
		return err
	}
	return nil
}

func (ssrv *SessSrv) GetStats() *stats.StatInfo {
	return ssrv.stats
}

func (ssrv *SessSrv) QueueLen() int64 {
	return ssrv.qlen.Read()
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

func (ssrv *SessSrv) GetRootCtx(uname sp.Tuname, aname string, sessid sessp.Tsession, clntid sp.TclntId) (fs.Dir, fs.CtxI) {
	return ssrv.dirover, ctx.MkCtx(uname, sessid, clntid, ssrv.sct)
}

// New session or new connection for existing session
func (ssrv *SessSrv) Register(cid sessp.Tclient, sid sessp.Tsession, conn sps.Conn) *serr.Err {
	db.DPrintf(db.SESSSRV, "Register sid %v %v\n", sid, conn)
	sess := ssrv.st.Alloc(cid, sid)
	return sess.SetConn(conn)
}

// Disassociate a connection with a session, and let it close gracefully.
func (ssrv *SessSrv) Unregister(cid sessp.Tclient, sid sessp.Tsession, conn sps.Conn) {
	// If this connection hasn't been associated with a session yet, return.
	if sid == sessp.NoSession {
		return
	}
	sess := ssrv.st.Alloc(cid, sid)
	sess.UnsetConn(conn)
}

func (ssrv *SessSrv) SrvFcall(fc *sessp.FcallMsg) {
	ssrv.qlen.Inc(1)
	s := sessp.Tsession(fc.Fc.Session)
	sess, ok := ssrv.st.Lookup(s)
	// Server-generated heartbeats will have session number 0. Pass them through.
	if !ok && s != 0 {
		db.DFatalf("SrvFcall: no session %v for req %v\n", s, fc)
	}
	if !ssrv.replicated {
		// If the fcall is a server-generated heartbeat, don't worry about
		// processing it sequentially on the session's thread.
		if s == 0 {
			ssrv.srvfcall(fc)
		} else if sessp.Tfcall(fc.Fc.Type) == sessp.TTwriteread {
			go func() {
				ssrv.srvfcall(fc)
			}()
		} else {
			sess.GetThread().Process(fc)
		}
	} else {
		ssrv.replSrv.Process(fc)
	}
}

func (ssrv *SessSrv) sendReply(request *sessp.FcallMsg, reply *sessp.FcallMsg, sess *sessstatesrv.Session) {
	// Store the reply in the reply cache.
	ok := sess.GetReplyTable().Put(request, reply)

	db.DPrintf(db.SESSSRV, "sendReply req %v rep %v ok %v", request, reply, ok)

	// If a client sent the request (seqno != 0) (as opposed to an
	// internally-generated detach or heartbeat), send reply.
	if request.Fc.Seqno != 0 && ok {
		sess.SendConn(reply)
	}
}

func (ssrv *SessSrv) srvfcall(fc *sessp.FcallMsg) {
	defer ssrv.qlen.Dec()
	// If this was a server-generated heartbeat message, heartbeat all of the
	// contained sessions, and then return immediately (no further processing is
	// necessary).
	s := sessp.Tsession(fc.Fc.Session)
	if s == 0 {
		ssrv.st.ProcessHeartbeats(fc.Msg.(*sp.Theartbeat))
		return
	}
	// If this is a replicated op received through raft (not
	// directly from a client), the first time Alloc is called
	// will be in this function, so the conn will be set to
	// nil. If it came from the client, the conn will already be
	// set.
	sess := ssrv.st.Alloc(sessp.Tclient(fc.Fc.Client), s)
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
	if replyFuture, ok := sess.GetReplyTable().Get(fc.Fc); ok {
		db.DPrintf(db.SESSSRV, "srvfcall %v reply in cache", fc)
		go func() {
			ssrv.sendReply(fc, replyFuture.Await(), sess)
		}()
		return
	}
	db.DPrintf(db.SESSSRV, "srvfcall %v reply not in cache", fc)
	if ok := sess.GetReplyTable().Register(fc); ok {
		db.DPrintf(db.REPLY_TABLE, "table: %v", sess.GetReplyTable())
		qlen := ssrv.QueueLen()
		ssrv.stats.Stats().Inc(fc.Msg.Type(), qlen)
		ssrv.fenceFcall(sess, fc)
	} else {
		db.DPrintf(db.SESSSRV, "srvfcall %v duplicate request dropped", fc)
	}
}

// Fence an fcall, if the call has a fence associated with it.  Note: don't fence blocking
// ops.
func (ssrv *SessSrv) fenceFcall(sess *sessstatesrv.Session, fc *sessp.FcallMsg) {
	db.DPrintf(db.FENCESRV, "fenceFcall %v fence %v\n", fc.Fc.Type, fc.Fc.Fence)
	if f, err := fencefs.CheckFence(ssrv.ffs, *fc.Tfence()); err != nil {
		msg := sp.MkRerror(err)
		reply := sessp.MakeFcallMsgReply(fc, msg)
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

func (ssrv *SessSrv) serve(sess *sessstatesrv.Session, fc *sessp.FcallMsg) {
	db.DPrintf(db.SESSSRV, "Dispatch request %v", fc)
	msg, data, close, rerror := sess.Dispatch(fc.Msg, fc.Data)
	db.DPrintf(db.SESSSRV, "Done dispatch request %v close? %v", fc, close)

	if rerror != nil {
		msg = rerror
	}

	reply := sessp.MakeFcallMsgReply(fc, msg)
	reply.Data = data

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
