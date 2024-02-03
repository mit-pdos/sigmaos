package netsrv

import (
	"bufio"
	"net"
	"sync"

	//db "sigmaos/debug"
	"sigmaos/demux"
	//"sigmaos/sessconn"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	//sps "sigmaos/sigmaprotsrv"
)

type NetSrvConn struct {
	sync.Mutex
	wg     *sync.WaitGroup
	conn   net.Conn
	dmx    *demux.DemuxSrv
	sessid sessp.Tsession
}

func NewNetSrvConn(srv *NetServer, conn net.Conn) *NetSrvConn {
	c := &NetSrvConn{
		wg:   &sync.WaitGroup{},
		conn: conn,
		dmx: demux.NewDemuxSrv(bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
			bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN), 2, srv.serve),
	}
	return c
}

func (c *NetSrvConn) Src() string {
	return c.conn.RemoteAddr().String()
}

func (c *NetSrvConn) Dst() string {
	return c.conn.LocalAddr().String()
}
