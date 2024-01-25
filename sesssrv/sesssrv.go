package sesssrv

import (
	"fmt"

	"sigmaos/clntcond"
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
	"sigmaos/sessp"
	"sigmaos/sessstatesrv"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
	"sigmaos/spcodec"
	"sigmaos/stats"
	"sigmaos/version"
	"sigmaos/watch"
)

//
// There is one SessSrv per server. The SessSrv has one protsrv per
// session (i.e., TCP connection).
//
// SessSrv has a table with all sess conds in use so that it can
// unblock threads that are waiting in a sess cond when a session
// closes.
//

type SessSrv struct {
	pe       *proc.ProcEnv
	srvpath  string
	dirunder fs.Dir
	dirover  *overlay.DirOverlay
	newps    sps.NewProtServer
	stats    *stats.StatInfo
	st       *sessstatesrv.SessionTable
	sm       *sessstatesrv.SessionMgr
	sct      *clntcond.ClntCondTable
	plt      *lockmap.PathLockTable
	wt       *watch.WatchTable
	vt       *version.VersionTable
	et       *ephemeralmap.EphemeralMap
	fencefs  fs.Dir
	srv      *netsrv.NetServer
	qlen     stats.Tcounter
}

func NewSessSrv(pe *proc.ProcEnv, srvpath string, root fs.Dir, addr *sp.Taddr, newps sps.NewProtServer, attachf sps.AttachClntF, detachf sps.DetachClntF, et *ephemeralmap.EphemeralMap, fencefs fs.Dir) *SessSrv {
	ssrv := &SessSrv{}
	ssrv.pe = pe
	ssrv.srvpath = srvpath
	ssrv.dirover = overlay.MkDirOverlay(root)
	ssrv.dirunder = root
	ssrv.newps = newps
	ssrv.et = et
	ssrv.stats = stats.NewStatsDev(ssrv.dirover)
	ssrv.st = sessstatesrv.NewSessionTable(newps, ssrv, attachf, detachf)
	ssrv.sct = clntcond.NewClntCondTable()
	ssrv.plt = lockmap.NewPathLockTable()
	ssrv.wt = watch.NewWatchTable(ssrv.sct)
	ssrv.vt = version.NewVersionTable()
	ssrv.vt.Insert(ssrv.dirover.Path())
	ssrv.fencefs = fencefs

	ssrv.dirover.Mount(sp.STATSD, ssrv.stats)

	ssrv.srv = netsrv.NewNetServer(pe, ssrv, addr, spcodec.WriteFcallAndData, spcodec.ReadUnmarshalFcallAndData)
	ssrv.sm = sessstatesrv.NewSessionMgr(ssrv.st, ssrv.SrvFcall)
	db.DPrintf(db.SESSSRV, "Listen on address: %v", ssrv.srv.MyAddr())
	return ssrv
}

func (ssrv *SessSrv) GetSrvPath() string {
	return ssrv.srvpath
}

func (ssrv *SessSrv) ProcEnv() *proc.ProcEnv {
	return ssrv.pe
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
		o, err := ssrv.dirover.Lookup(ctx.NewCtxNull(), path[0])
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
		return serr.NewErr(serr.TErrNotfound, sid)
	}
	sess.RegisterDetachSess(f)
	return nil
}

func (ssrv *SessSrv) MyAddr() *sp.Taddr {
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

func (ssrv *SessSrv) GetSessionCondTable() *clntcond.ClntCondTable {
	return ssrv.sct
}

func (ssrv *SessSrv) GetRootCtx(principal *sp.Tprincipal, aname string, sessid sessp.Tsession, clntid sp.TclntId) (fs.Dir, fs.CtxI) {
	return ssrv.dirover, ctx.NewCtx(principal, sessid, clntid, ssrv.sct, ssrv.fencefs)
}

// New session or new connection for existing session
func (ssrv *SessSrv) Register(sid sessp.Tsession, conn sps.Conn) *serr.Err {
	db.DPrintf(db.SESSSRV, "Register sid %v %v\n", sid, conn)
	sess := ssrv.st.Alloc(sid)
	return sess.SetConn(conn)
}

// Disassociate a connection with a session, and let it close gracefully.
func (ssrv *SessSrv) Unregister(sid sessp.Tsession, conn sps.Conn) {
	// If this connection hasn't been associated with a session yet, return.
	if sid == sessp.NoSession {
		return
	}
	sess := ssrv.st.Alloc(sid)
	sess.UnsetConn(conn)
}

func (ssrv *SessSrv) SrvFcall(fc *sessp.FcallMsg) *serr.Err {
	ssrv.qlen.Inc(1)
	s := sessp.Tsession(fc.Fc.Session)
	_, ok := ssrv.st.Lookup(s)
	// Server-generated heartbeats will have session number 0. Pass them through.
	if !ok && s != 0 {
		db.DPrintf(db.ERROR, "SrvFcall: no session %v for req %v", s, fc)
		return serr.NewErrError(fmt.Errorf("Error: no session %v for req %v", s, fc))
	}
	// If the fcall is a server-generated heartbeat, it won't block;
	// don't start a new thread.
	if s == 0 {
		ssrv.srvfcall(fc)
	} else {
		go func() {
			ssrv.srvfcall(fc)
		}()
	}
	return nil
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
	sess := ssrv.st.Alloc(s)
	qlen := ssrv.QueueLen()
	ssrv.stats.Stats().Inc(fc.Msg.Type(), qlen)
	ssrv.serve(sess, fc)
}

func (ssrv *SessSrv) serve(sess *sessstatesrv.Session, fc *sessp.FcallMsg) {
	db.DPrintf(db.SESSSRV, "Dispatch request %v", fc)
	msg, data, rerror, op, clntid := sess.Dispatch(fc.Msg, fc.Data)
	db.DPrintf(db.SESSSRV, "Done dispatch request %v", fc)

	if rerror != nil {
		db.DPrintf(db.SESSSRV, "%v: Dispatch %v rerror %v", sess.Sid, fc, rerror)
		msg = rerror
	}

	reply := sessp.NewFcallMsgReply(fc, msg)
	reply.Data = data
	ssrv.sendReply(fc, reply, sess)

	switch op {
	case sessstatesrv.TSESS_DEL:
		sess.DelClnt(clntid)
		ssrv.st.DelLastClnt(clntid)
	case sessstatesrv.TSESS_ADD:
		sess.AddClnt(clntid)
		ssrv.st.AddLastClnt(clntid, sess.Sid)
	case sessstatesrv.TSESS_NONE:
	}
}

func (ssrv *SessSrv) PartitionClient(permanent bool) {
	if permanent {
		ssrv.sm.DisconnectClient()
	} else {
		ssrv.sm.CloseConn()
	}
}
