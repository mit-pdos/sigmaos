package sesssrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/ephemeralmap"
	"sigmaos/fs"
	"sigmaos/lockmap"
	"sigmaos/netsrv"
	"sigmaos/overlaydir"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sesscond"
	"sigmaos/sessp"
	"sigmaos/sessstatesrv"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
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
	addr     string
	dirunder fs.Dir
	dirover  *overlay.DirOverlay
	mkps     sps.MkProtServer
	stats    *stats.StatInfo
	st       *sessstatesrv.SessionTable
	sm       *sessstatesrv.SessionMgr
	sct      *sesscond.SessCondTable
	tmt      *threadmgr.ThreadMgrTable
	plt      *lockmap.PathLockTable
	wt       *watch.WatchTable
	vt       *version.VersionTable
	et       *ephemeralmap.EphemeralMap
	fencefs  fs.Dir
	srv      *netsrv.NetServer
	qlen     stats.Tcounter
}

func MakeSessSrv(root fs.Dir, addr string, mkps sps.MkProtServer, attachf sps.AttachClntF, detachf sps.DetachClntF, et *ephemeralmap.EphemeralMap, fencefs fs.Dir) *SessSrv {
	ssrv := &SessSrv{}
	ssrv.dirover = overlay.NewDirOverlay(root)
	ssrv.dirunder = root
	ssrv.addr = addr
	ssrv.mkps = mkps
	ssrv.et = et
	ssrv.stats = stats.MkStatsDev(ssrv.dirover)
	ssrv.tmt = threadmgr.MakeThreadMgrTable(ssrv.srvfcall)
	ssrv.st = sessstatesrv.MakeSessionTable(mkps, ssrv, ssrv.tmt, attachf, detachf)
	ssrv.sct = sesscond.MakeSessCondTable(ssrv.st)
	ssrv.plt = lockmap.MkPathLockTable()
	ssrv.wt = watch.MkWatchTable(ssrv.sct)
	ssrv.vt = version.MkVersionTable()
	ssrv.vt.Insert(ssrv.dirover.Path())
	ssrv.fencefs = fencefs

	ssrv.dirover.Mount(sp.STATSD, ssrv.stats)

	ssrv.srv = netsrv.MakeNetServer(ssrv, addr, spcodec.WriteFcallAndData, spcodec.ReadUnmarshalFcallAndData)
	ssrv.sm = sessstatesrv.MakeSessionMgr(ssrv.st, ssrv.SrvFcall)
	db.DPrintf(db.SESSSRV, "Listen on address: %v", ssrv.srv.MyAddr())
	return ssrv
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

func (ssrv *SessSrv) GetRootCtx(uname sp.Tuname, aname string, sessid sessp.Tsession, clntid sp.TclntId) (fs.Dir, fs.CtxI) {
	return ssrv.dirover, ctx.MkCtx(uname, sessid, clntid, ssrv.sct, ssrv.fencefs)
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
}

func (ssrv *SessSrv) sendReply(request *sessp.FcallMsg, reply *sessp.FcallMsg, sess *sessstatesrv.Session) {
	db.DPrintf(db.SESSSRV, "sendReply req %v rep %v", request, reply)

	// If a client sent the request (seqno != 0) (as opposed to an
	// internally-generated detach or heartbeat), send reply.
	if request.Fc.Seqno != 0 {
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
	qlen := ssrv.QueueLen()
	ssrv.stats.Stats().Inc(fc.Msg.Type(), qlen)
	ssrv.serve(sess, fc)
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
