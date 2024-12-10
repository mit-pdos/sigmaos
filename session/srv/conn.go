package srv

import (
	"net"
	"sync"

	"sigmaos/util/io/demux"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	spcodec "sigmaos/session/codec"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type netConn struct {
	sync.Mutex

	p      *sp.Tprincipal
	dmx    *demux.DemuxSrv
	conn   net.Conn
	ssrv   *SessSrv
	sessid sessp.Tsession
	sess   *Session
}

func (nc *netConn) getSess(sid sessp.Tsession) *Session {
	nc.Lock()
	defer nc.Unlock()
	if nc.sessid == sessp.NoSession {
		nc.sessid = sid
	}
	if nc.sessid != sid {
		db.DFatalf("Bad sid %v sess associated with conn %v\n", nc.sessid, nc)
	}
	return nc.sess
}

func (nc *netConn) setSess(sess *Session) {
	nc.Lock()
	defer nc.Unlock()
	nc.sess = sess
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
	if nc.conn != nil {
		db.DPrintf(db.CRASH, "CloseConnTest: close conn for sid %v\n", nc.sessid)
		return nc.conn.Close()
	}
	return nil
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
	nc.sess.UnsetConn(nc)
}

func (nc *netConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	fcm := c.(*sessp.PartMarshaledMsg)
	s := sessp.Tsession(fcm.Fcm.Session())
	sess := nc.getSess(s)
	if sess == nil {
		sess = nc.ssrv.st.Alloc(nc.p, s, nc)
		nc.setSess(sess)
	}
	if err := spcodec.UnmarshalMsg(fcm); err != nil {
		return nil, err
	}
	db.DPrintf(db.NET_LAT, "ReadCall fm %v\n", fcm)
	rep := nc.ssrv.serve(nc.sess, fcm.Fcm)
	pmfc := spcodec.NewPartMarshaledMsg(rep)
	return pmfc, nil
}
