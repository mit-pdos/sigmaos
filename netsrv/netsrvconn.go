package netsrv

import (
	"bufio"
	"net"
	"sync"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type NetSrvConn struct {
	sync.Mutex
	wg      *sync.WaitGroup
	conn    net.Conn
	dmx     *demux.DemuxSrv
	sesssrv sps.SessServer
	sessid  sessp.Tsession
}

func NewNetSrvConn(srv *NetServer, conn net.Conn) *NetSrvConn {
	c := &NetSrvConn{
		wg:      &sync.WaitGroup{},
		conn:    conn,
		sesssrv: srv.sesssrv,
		sessid:  sessp.NoSession,
	}
	dmx := demux.NewDemuxSrv(bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN), srv.readCall, srv.writeCall, c)
	c.dmx = dmx
	return c
}

func (c *NetSrvConn) Src() string {
	return c.conn.RemoteAddr().String()
}

func (c *NetSrvConn) Dst() string {
	return c.conn.LocalAddr().String()
}

// If no sid associated with c, then associated sid with c.
func (c *NetSrvConn) CondSet(sid sessp.Tsession) sessp.Tsession {
	c.Lock()
	defer c.Unlock()
	if c.sessid == sessp.NoSession {
		c.sessid = sid
	}
	return c.sessid
}

func (c *NetSrvConn) GetSessId() sessp.Tsession {
	c.Lock()
	defer c.Unlock()
	return c.sessid
}

func (c *NetSrvConn) Close() error {
	db.DPrintf(db.NETSRV, "Close %v\n", c)
	if err := c.conn.Close(); err != nil {
		db.DPrintf(db.ALWAYS, "NetSrvConn.Close: err %v\n", err)
	}
	return c.dmx.Close()
}

func (c *NetSrvConn) IsClosed() bool {
	db.DPrintf(db.NETSRV, "IsClosed %v\n", c.sessid)
	return c.dmx.IsClosed()
}

func (c *NetSrvConn) CloseConnTest() error {
	db.DPrintf(db.CRASH, "CloseConnTest: close conn for sid %v\n", c.sessid)
	return c.conn.Close()
}

func (c *NetSrvConn) ReportError(err error) {
	db.DPrintf(db.NETSRV, "ReportError %v %v\n", c.sessid, err)
	c.sesssrv.ReportError(c, err)
}

func (c *NetSrvConn) ServeRequest(fc demux.CallI) (demux.CallI, *serr.Err) {
	return c.sesssrv.ServeRequest(c, fc)
}