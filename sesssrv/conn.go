package sesssrv

import (
	"net"
	"sync"

	"sigmaos/demux"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/spcodec"

	db "sigmaos/debug"
)

type netConn struct {
	sync.Mutex

	dmx    *demux.DemuxSrv
	conn   net.Conn
	ssrv   *SessSrv
	sessid sessp.Tsession
}

// If no sid associated with nc, then associated sid with nc.
func (nc *netConn) CondSet(sid sessp.Tsession) sessp.Tsession {
	nc.Lock()
	defer nc.Unlock()
	if nc.sessid == sessp.NoSession {
		nc.sessid = sid
	}
	return nc.sessid
}

func (nc *netConn) GetSessId() sessp.Tsession {
	nc.Lock()
	defer nc.Unlock()
	return nc.sessid
}

func (nc *netConn) Close() error {
	db.DPrintf(db.SESSSRV, "Close %v\n", nc)
	if err := nc.conn.Close(); err != nil {
		db.DPrintf(db.ALWAYS, "NetSrvConn.Close: err %v\n", err)
	}
	return nc.dmx.Close()
}

func (nc *netConn) IsClosed() bool {
	db.DPrintf(db.NETSRV, "IsClosed %v\n", nc.sessid)
	return nc.dmx.IsClosed()
}

func (nc *netConn) CloseConnTest() error {
	db.DPrintf(db.CRASH, "CloseConnTest: close conn for sid %v\n", nc.sessid)
	return nc.conn.Close()
}

func (nc *netConn) Src() string {
	return nc.conn.RemoteAddr().String()
}

func (nc *netConn) Dst() string {
	return nc.conn.LocalAddr().String()
}

func (nc *netConn) ReportError(err error) {
	db.DPrintf(db.SESSSRV, "ReportError %v err %v\n", nc.conn, err)

	// Disassociate a connection with a session, and let it close gracefully.
	sid := nc.sessid
	if sid == sessp.NoSession {
		return
	}
	sess := nc.ssrv.st.Alloc(sid)
	sess.UnsetConn(nc)
}

func (nc *netConn) ServeRequest(fc demux.CallI) (demux.CallI, *serr.Err) {
	rep := nc.ssrv.srvFcall(nc, fc.(*sessp.FcallMsg))
	pmfc := spcodec.NewPartMarshaledMsg(rep)
	return pmfc, nil
}
