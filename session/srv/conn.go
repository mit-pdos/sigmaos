package srv

import (
	//"net"
	"sync"

	"sigmaos/serr"
	spcodec "sigmaos/session/codec"
	sessp "sigmaos/session/proto"
	"sigmaos/util/io/demux"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type netConn struct {
	sync.Mutex

	p      *sp.Tprincipal
	dmx    *demux.DemuxSrv
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
	return nc.dmx.Close()
}

func (nc *netConn) IsClosed() bool {
	db.DPrintf(db.NETSRV, "IsClosed %v\n", nc.sessid)
	return nc.dmx.IsClosed()
}

func (nc *netConn) ReportError(err error) {
	db.DPrintf(db.SESSSRV, "ReportError %v err %v\n", nc, err)

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
