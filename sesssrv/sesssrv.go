// The sesssrv package dispatches incoming calls on a session to a
// protsrv for that session.  The clients on a session share an fid
// table.
package sesssrv

import (
	"bufio"
	"net"

	"sigmaos/clntcond"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
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
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
	"sigmaos/spcodec"
	"sigmaos/stats"
	"sigmaos/version"
	"sigmaos/watch"
)

type NewSessionI interface {
	NewSession(sessp.Tsession) sps.Protsrv
}

//
// SessSrv has a table with all sess conds in use so that it can
// unblock threads that are waiting in a sess cond when a session
// closes.
//

type SessSrv struct {
	pe       *proc.ProcEnv
	dirunder fs.Dir
	dirover  *overlay.DirOverlay
	stats    *stats.StatInfo
	st       *SessionTable
	sm       *SessionMgr
	sct      *clntcond.ClntCondTable
	plt      *lockmap.PathLockTable
	wt       *watch.WatchTable
	vt       *version.VersionTable
	et       *ephemeralmap.EphemeralMap
	fencefs  fs.Dir
	srv      *netsrv.NetServer
	qlen     stats.Tcounter
}

func NewSessSrv(pe *proc.ProcEnv, root fs.Dir, addr *sp.Taddr, newSess NewSessionI, et *ephemeralmap.EphemeralMap, fencefs fs.Dir) *SessSrv {
	ssrv := &SessSrv{}
	ssrv.pe = pe
	ssrv.dirover = overlay.MkDirOverlay(root)
	ssrv.dirunder = root
	ssrv.et = et
	ssrv.stats = stats.NewStatsDev(ssrv.dirover)
	ssrv.st = NewSessionTable(newSess)
	ssrv.sct = clntcond.NewClntCondTable()
	ssrv.plt = lockmap.NewPathLockTable()
	ssrv.wt = watch.NewWatchTable(ssrv.sct)
	ssrv.vt = version.NewVersionTable()
	ssrv.vt.Insert(ssrv.dirover.Path())
	ssrv.fencefs = fencefs

	ssrv.dirover.Mount(sp.STATSD, ssrv.stats)

	ssrv.srv = netsrv.NewNetServer(pe, addr, ssrv)
	ssrv.sm = NewSessionMgr(ssrv.st, ssrv.SrvFcall)
	db.DPrintf(db.SESSSRV, "Listen on address: %v", ssrv.srv.MyAddr())
	return ssrv
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

func (ssrv *SessSrv) GetRootCtx(uname sp.Tuname, aname string, sessid sessp.Tsession, clntid sp.TclntId) (fs.Dir, fs.CtxI) {
	return ssrv.dirover, ctx.NewCtx(uname, sessid, clntid, ssrv.sct, ssrv.fencefs)
}

func (ssrv *SessSrv) GetSessionTable() *SessionTable {
	return ssrv.st
}

func (ssrv *SessSrv) NewConn(conn net.Conn) *demux.DemuxSrv {
	nc := &netConn{conn: conn, ssrv: ssrv, sessid: sessp.NoSession}
	br := bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN)
	wr := bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN)
	nc.dmx = demux.NewDemuxSrv(br, wr, spcodec.ReadCall, spcodec.WriteCall, nc)
	return nc.dmx
}

// Serve server-generated fcalls.
func (ssrv *SessSrv) SrvFcall(fc *sessp.FcallMsg) *sessp.FcallMsg {
	s := sessp.Tsession(fc.Fc.Session)
	sess := ssrv.st.Alloc(s)
	return ssrv.serve(sess, fc)
}

func (ssrv *SessSrv) srvFcall(nc *netConn, fc *sessp.FcallMsg) *sessp.FcallMsg {
	s := sessp.Tsession(fc.Fc.Session)
	if nc.CondSet(s) != s {
		db.DFatalf("Bad sid %v sess associated with conn %v\n", nc.sessid, nc)
	}
	sess := ssrv.st.Alloc(s)
	sess.SetConn(nc)
	return ssrv.serve(sess, fc)
}

func (ssrv *SessSrv) serve(sess *Session, fc *sessp.FcallMsg) *sessp.FcallMsg {
	ssrv.qlen.Inc(1)
	defer ssrv.qlen.Dec()

	qlen := ssrv.QueueLen()
	ssrv.stats.Stats().Inc(fc.Msg.Type(), qlen)

	db.DPrintf(db.SESSSRV, "Dispatch request %v", fc)
	msg, iov, rerror, op, clntid := sess.Dispatch(fc.Msg, fc.Iov)
	db.DPrintf(db.SESSSRV, "Done dispatch request %v", fc)

	if rerror != nil {
		db.DPrintf(db.SESSSRV, "%v: Dispatch %v rerror %v", sess.Sid, fc, rerror)
		msg = rerror
	}

	reply := sessp.NewFcallMsgReply(fc, msg)
	reply.Iov = iov

	switch op {
	case TSESS_DEL:
		sess.DelClnt(clntid)
		ssrv.st.DelLastClnt(clntid)
	case TSESS_ADD:
		sess.AddClnt(clntid)
		ssrv.st.AddLastClnt(clntid, sess.Sid)
	case TSESS_NONE:
	}

	return reply
}

func (ssrv *SessSrv) PartitionClient(permanent bool) {
	if permanent {
		ssrv.sm.DisconnectClient()
	} else {
		ssrv.sm.CloseConn()
	}
}
