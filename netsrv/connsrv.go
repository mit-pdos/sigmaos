package netsrv

import (
	"bufio"
	"net"
	"sync"

	"time"

	db "sigmaos/debug"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type SrvConn struct {
	*sync.Mutex
	wg             *sync.WaitGroup
	conn           net.Conn
	closed         bool
	sesssrv        sps.SessServer
	br             *bufio.Reader
	bw             *bufio.Writer
	replies        chan *sessp.FcallMsg
	marshalframe   MarshalF
	unmarshalframe UnmarshalF
	clid           sessp.Tclient
	sessid         sessp.Tsession
}

func MakeSrvConn(srv *NetServer, conn net.Conn) *SrvConn {
	c := &SrvConn{
		&sync.Mutex{},
		&sync.WaitGroup{},
		conn,
		false,
		srv.sesssrv,
		bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
		make(chan *sessp.FcallMsg),
		srv.marshal,
		srv.unmarshal,
		0,
		0,
	}
	go c.writer()
	go c.reader()
	return c
}

func (c *SrvConn) Close() {
	db.DPrintf(db.NETSRV, "Cli %v Sess %v Prepare to close conn and replies %p", c.clid, c.sessid, c.replies)

	c.Lock()
	defer c.Unlock()

	// Close may be called twice, e.g. if the connection breaks as the session is
	// closing.
	if c.closed {
		return
	}

	c.closed = true
	// Wait for all senders on the replies channel before closing it. The reader
	// will then exit and close the TCP connection.
	go func() {
		c.wg.Wait()
		db.DPrintf(db.NETSRV, "Cli %v Sess %v Close replies chan %p", c.clid, c.sessid, c.replies)
		close(c.replies)
	}()
}

func (c *SrvConn) IsClosed() bool {
	c.Lock()
	defer c.Unlock()

	return c.closed
}

// For testing purposes.
func (c *SrvConn) CloseConnTest() {
	c.Lock()
	defer c.Unlock()

	c.conn.Close()
}

func (c *SrvConn) Src() string {
	return c.conn.RemoteAddr().String()
}

func (c *SrvConn) Dst() string {
	return c.conn.LocalAddr().String()
}

// Get the reply channel in order to send an sessp. If this function is called,
// the caller *must* send something on the replies channel, otherwise the
// WaitGroup counter will be wrong. This ensures that the channel isn't closed
// out from under a sender's feet.
func (c *SrvConn) GetReplyChan() chan *sessp.FcallMsg {
	// XXX grab lock?
	c.wg.Add(1)
	return c.replies
}

func (c *SrvConn) reader() {
	db.DPrintf(db.NETSRV, "Cli %v Sess %v (%v) Reader conn from %v\n", c.clid, c.sessid, c.Dst(), c.Src())
	for {
		fc, err := c.unmarshalframe(c.br)
		if err != nil {
			db.DPrintf(db.NETSRV_ERR, "%v reader from %v: bad frame: %v", c.sessid, c.Src(), err)
			return
		}
		db.DPrintf(db.NETSRV, "srv req %v data %d\n", fc, len(fc.Data))
		if c.sessid == 0 {
			c.sessid = sessp.Tsession(fc.Session())
			c.clid = fc.Client()
			if err := c.sesssrv.Register(c.clid, c.sessid, c); err != nil {
				db.DPrintf(db.NETSRV_ERR, "Cli %v Sess %v closed\n", c.clid, c.sessid)
				// Push a message telling the client that it's session has been closed,
				// and it shouldn't try to reconnect.
				fm := sessp.MakeFcallMsgReply(fc, sp.MkRerror(err))
				c.GetReplyChan() <- fm
				close(c.replies)
				return
			} else {
				// If we successfully registered, we'll have to unregister once the
				// connection breaks. This function tells the underlying sesssrv that
				// the connection has broken.
				defer c.sesssrv.Unregister(c.clid, c.sessid, c)
			}
		} else if c.sessid != sessp.Tsession(fc.Session()) {
			db.DFatalf("reader: two sess (%v and %v) on conn?\n", c.sessid, fc.Session())
		}
		c.sesssrv.SrvFcall(fc)
	}
}

func (c *SrvConn) write(fm *sessp.FcallMsg) {
	// Mark that the sender is no longer waiting to send on the replies channel.
	c.wg.Done()
	db.DPrintf(db.NETSRV, "rep %v\n", fm)
	start := time.Now()
	if err := c.marshalframe(fm, c.bw); err != nil {
		db.DPrintf(db.NETSRV_ERR, "%v writer %v err %v\n", c.sessid, c.Src(), err)
		return
	}
	if time.Since(start) > 20*time.Millisecond {
		db.DPrintf(db.ALWAYS, "Long marshal time %v", time.Since(start))
	}
}

func (c *SrvConn) writer() {
	// Close the TCP connection once we return.
	defer c.conn.Close()
	for {
		fm, ok := <-c.replies
		if !ok {
			db.DPrintf(db.NETSRV, "%v writer: close conn from %v\n", c.sessid, c.Src())
			return
		}
		c.write(fm)
	}
}
