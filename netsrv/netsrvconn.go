package netsrv

import (
	"bufio"
	"net"
	"sync"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
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
		bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN), 2, c)
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
	c.Lock()
	defer c.Unlock()

	db.DPrintf(db.NETSRV, "Close %v\n", c)
	c.conn.Close()
	return c.dmx.Close()
}

func (c *NetSrvConn) IsClosed() bool {
	db.DPrintf(db.NETSRV, "IsClosed %v\n", c)
	return c.dmx.IsClosed()
}

func (c *NetSrvConn) CloseConnTest() error {
	db.DPrintf(db.CRASH, "CloseConnTest %v\n", c)
	return c.conn.Close()
}

func (c *NetSrvConn) ReportError(err error) {
	db.DPrintf(db.NETSRV, "ReportError %v %v\n", c, err)
	c.sesssrv.ReportError(c, err)
}

func (c *NetSrvConn) ServeRequest(req []frame.Tframe) ([]frame.Tframe, *serr.Err) {
	return c.sesssrv.ServeRequest(c, req)
}
