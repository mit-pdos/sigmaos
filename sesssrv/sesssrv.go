// The sesssrv package dispatches incoming calls on a session to a
// protsrv for that session.  The clients on a session share an fid
// table.
package sesssrv

import (
	"net"

	db "sigmaos/debug"
	"sigmaos/demux"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/netsrv"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
	"sigmaos/spcodec"
	"sigmaos/stats"
)

type NewSessionI interface {
	NewSession(*sp.Tprincipal, sessp.Tsession) sps.Protsrv
}

//
// SessSrv has a table with all sess conds in use so that it can
// unblock threads that are waiting in a sess cond when a session
// closes.
//

type SessSrv struct {
	pe    *proc.ProcEnv
	st    *sessionTable
	sm    *sessionMgr
	srv   *netsrv.NetServer
	stats *stats.StatInode
	qlen  stats.Tcounter
}

func NewSessSrv(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, addr *sp.Taddr, stats *stats.StatInode, newSess NewSessionI) *SessSrv {
	ssrv := &SessSrv{
		pe:    pe,
		stats: stats,
		st:    newSessionTable(newSess),
	}
	ssrv.srv = netsrv.NewNetServer(pe, npc, addr, ssrv)
	ssrv.sm = newSessionMgr(ssrv.st, ssrv.srvFcall)
	db.DPrintf(db.SESSSRV, "Listen on address: %v", ssrv.srv.GetEndpoint())
	return ssrv
}

func (ssrv *SessSrv) ProcEnv() *proc.ProcEnv {
	return ssrv.pe
}

func (sssrv *SessSrv) RegisterDetachSess(f sps.DetachSessF, sid sessp.Tsession) *serr.Err {
	sess, ok := sssrv.st.Lookup(sid)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, sid)
	}
	sess.RegisterDetachSess(f)
	return nil
}

func (ssrv *SessSrv) GetEndpoint() *sp.Tendpoint {
	return ssrv.srv.GetEndpoint()
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

func (ssrv *SessSrv) QueueLen() int64 {
	return ssrv.qlen.Load()
}

// for testing
func (ssrv *SessSrv) GetSessionTable() *sessionTable {
	return ssrv.st
}

func (ssrv *SessSrv) NewConn(p *sp.Tprincipal, conn net.Conn) *demux.DemuxSrv {
	nc := &netConn{
		p:      p,
		conn:   conn,
		ssrv:   ssrv,
		sessid: sessp.NoSession,
	}
	iovm := demux.NewIoVecMap()
	nc.dmx = demux.NewDemuxSrv(nc, spcodec.NewTransport(conn, iovm))
	return nc.dmx
}

// Serve server-generated fcalls.
func (ssrv *SessSrv) srvFcall(sess *Session, fc *sessp.FcallMsg) *sessp.FcallMsg {
	return ssrv.serve(sess, fc)
}

func (ssrv *SessSrv) serve(sess *Session, fc *sessp.FcallMsg) *sessp.FcallMsg {
	stats.Inc(&ssrv.qlen, 1)
	defer stats.Dec(&ssrv.qlen)

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
	case sps.TSESS_DEL:
		sess.DelClnt(clntid)
		ssrv.st.DelLastClnt(clntid)
	case sps.TSESS_ADD:
		sess.AddClnt(clntid)
		ssrv.st.AddLastClnt(clntid, sess)
	case sps.TSESS_NONE:
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
