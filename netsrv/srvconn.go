package netsrv

import (
	"bufio"
	"net"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

const (
	Msglen = 64 * 1024
)

type SrvConn struct {
	*sync.Mutex
	wg         *sync.WaitGroup
	conn       net.Conn
	closed     bool
	wireCompat bool
	br         *bufio.Reader
	bw         *bufio.Writer
	replies    chan *np.Fcall
	sesssrv    np.SessServer
	sessid     np.Tsession
}

func MakeSrvConn(srv *NetServer, conn net.Conn) *SrvConn {
	c := &SrvConn{
		&sync.Mutex{},
		&sync.WaitGroup{},
		conn,
		false,
		srv.wireCompat,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		srv.sesssrv,
		0,
	}
	go c.writer()
	go c.reader()
	return c
}

func (c *SrvConn) Close() {
	db.DPrintf("NETSRV", "%v Prepare to close conn and replies %p", c.sessid, c.replies)

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
		db.DPrintf("NETSRV", "%v Close replies chan %p", c.sessid, c.replies)
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

// Get the reply channel in order to send an fcall. If this function is called,
// the caller *must* send something on the replies channel, otherwise the
// WaitGroup counter will be wrong. This ensures that the channel isn't closed
// out from under a sender's feet.
func (c *SrvConn) GetReplyC() chan *np.Fcall {
	// XXX grab lock?
	c.wg.Add(1)
	return c.replies
}

func (c *SrvConn) reader() {
	db.DPrintf("NETSRV", "%v (%v) Reader conn from %v\n", c.sessid, c.Dst(), c.Src())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			db.DPrintf("NETSRV_ERR", "%v ReadFrame err %v\n", c.sessid, err)
			return
		}
		var fcall *np.Fcall
		if c.wireCompat {
			fcall, err = npcodec.UnmarshalFcallWireCompat(frame)
		} else {
			fcall, err = npcodec.UnmarshalFcall(frame)
		}
		if err != nil {
			db.DPrintf("NETSRV_ERR", "%v reader from %v: bad fcall: ", c.sessid, c.Src(), err)
			return
		}
		db.DPrintf("NETSRV", "srv req %v\n", fcall)
		if c.sessid == 0 {
			c.sessid = fcall.Session
			if err := c.sesssrv.Register(fcall.Session, c); err != nil {
				db.DPrintf("NETSRV_ERR", "Sess %v closed\n", c.sessid)
				// Push a message telling the client that it's session has been closed,
				// and it shouldn't try to reconnect.
				c.wg.Add(1)
				fc := np.MakeFcallReply(fcall, err.Rerror())
				c.replies <- fc
				close(c.replies)
				return
			} else {
				// If we successfully registered, we'll have to unregister once the
				// connection breaks. This function tells the underlying sesssrv that
				// the connection has broken.
				defer c.sesssrv.Unregister(c.sessid, c)
			}
		} else if c.sessid != fcall.Session {
			db.DFatalf("reader: two sess (%v and %v) on conn?\n", c.sessid, fcall.Session)
		}
		c.sesssrv.SrvFcall(fcall)
	}
}

func (c *SrvConn) writer() {
	// Close the TCP connection once we return.
	defer c.conn.Close()
	for {
		fcall, ok := <-c.replies
		if !ok {
			db.DPrintf("NETSRV", "%v writer: close conn from %v\n", c.sessid, c.Src())
			return
		}
		// Mark that the sender is no longer waiting to send on the replies channel.
		c.wg.Done()
		db.DPrintf("NETSRV", "rep %v\n", fcall)
		var writableFcall np.WritableFcall
		if c.wireCompat {
			writableFcall = fcall.ToWireCompatible()
		} else {
			writableFcall = fcall
		}
		if err := npcodec.MarshalFcall(writableFcall, c.bw); err != nil {
			db.DPrintf("NETSRV_ERR", "%v writer %v err %v\n", c.sessid, c.Src(), err)
			continue
		}
		if error := c.bw.Flush(); error != nil {
			db.DPrintf("NETSRV_ERR", "flush %v to %v err %v", fcall, c.Src(), error)
		}
	}
}
