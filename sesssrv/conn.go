package sesssrv

import (
	"fmt"
	"net"
	"runtime"
	"sync"

	"sigmaos/demux"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/spcodec"

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

func printAllStacks() {
	db.DPrintf(db.ALWAYS, "Printing Stacks")

	buf := make([]byte, 1<<20) // 1 MB buffer to hold stack traces
	n := runtime.Stack(buf, true)
	db.DPrintf(db.ALWAYS, "%s", string(buf[:n]))
}
func (nc *netConn) getSess(sid sessp.Tsession) (*Session, error) {
	nc.Lock()
	defer nc.Unlock()
	if nc.sessid == sessp.NoSession {
		nc.sessid = sid
	}
	if nc.sessid != sid {
		db.DPrintf(db.ERROR, "Bad sid %v sess netconn: %p requestedSess %v\n", nc.sessid, nc, sid)
		//	printAllStacks()
		//db.DFatalf("Bad sid %v sess associated with conn %v requestedSess %v\n", nc.sessid, nc, sid)
		return nil, fmt.Errorf("bad sid %v\n", nc.sessid)
	}
	return nc.sess, nil
}

func (nc *netConn) setSess(sess *Session) {
	nc.Lock()
	defer nc.Unlock()
	nc.sess = sess
}

func (nc *netConn) Close() error {
	db.DPrintf(db.SESSSRV, "Close %p\n", nc)
	//time.Sleep(100 * time.Millisecond)
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
	db.DPrintf(db.SESSSRV, "%v ReportError %v err %v\n", nc.sessid, nc.conn, err)

	// Disassociate a connection with a session, and let it close gracefully.
	sid := nc.sessid
	if sid == sessp.NoSession {
		return
	}
	//time.Sleep(1000 * time.Millisecond)
	nc.sess.UnsetConn(nc)
	//db.DPrintf(db.SESSSRV, "%v ReportError %v err %v\n", nc.sessid, nc.conn, err)
}

func (nc *netConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	//db.DPrintf(db.SESSSRV, "serving %v %p\n", nc.sessid, nc)
	fcm := c.(*sessp.PartMarshaledMsg)
	s := sessp.Tsession(fcm.Fcm.Session())
	sess, err := nc.getSess(s)
	if err != nil {
		//	db.DPrintf(db.ERROR, "getSess Error: %v", fcm)
		return nil, serr.NewErrError(err)
	}
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
