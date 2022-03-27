package netsrv

import (
	"bufio"
	"log"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/protsrv"
)

const (
	Msglen = 64 * 1024
)

type SrvConn struct {
	conn       net.Conn
	wireCompat bool
	br         *bufio.Reader
	bw         *bufio.Writer
	replies    chan *np.Fcall
	protsrv    protsrv.FsServer
	sessid     np.Tsession
}

func MakeSrvConn(srv *NetServer, conn net.Conn) *SrvConn {
	c := &SrvConn{conn,
		srv.wireCompat,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		srv.fssrv,
		0,
	}
	go c.writer()
	go c.reader()
	return c
}

func (c *SrvConn) Src() string {
	return c.conn.RemoteAddr().String()
}

func (c *SrvConn) Dst() string {
	return c.conn.LocalAddr().String()
}

func (c *SrvConn) reader() {
	db.DLPrintf("NETSRV", "%v (%v)Reader conn from %v\n", c.sessid, c.Dst(), c.Src())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			db.DLPrintf("NETSRV", "%v Peer %v closed/erred %v\n", c.sessid, c.Src(), err)

			// If the sessid hasn't been set, we haven't received any valid ops yet,
			// so the session has not been added to the session table. If this is the
			// case, don't close the session (there is nothing to close).
			if c.sessid != 0 {
				c.protsrv.CloseSession(c.sessid)
			}

			// close the reply channel, so that conn writer() terminates
			db.DLPrintf("NETSRV", "%v Reader: close replies for %v\n", c.sessid, c.Src())
			return
		}
		var fcall *np.Fcall
		if c.wireCompat {
			fcall, err = npcodec.UnmarshalFcallWireCompat(frame)
		} else {
			fcall, err = npcodec.UnmarshalFcall(frame)
		}
		if err != nil {
			db.DLPrintf("NETSRV_ERR", "%v reader from %v: bad fcall: ", c.sessid, c.Src(), err)
		} else {
			db.DLPrintf("NETSRV", "srv req %v\n", fcall)
			if c.sessid == 0 {
				c.sessid = fcall.Session
			} else if c.sessid != fcall.Session {
				log.Fatal("FATAL reader: two sess (%v and %v) on conn?\n", c.sessid, fcall.Session)
			}
			c.protsrv.Process(fcall, c.replies)
		}
	}
}

// XXX Should we close with other error conditions?
func (c *SrvConn) writer() {
	for {
		fcall, ok := <-c.replies
		if !ok {
			db.DLPrintf("NETSRV", "%v writer: close conn from %v\n", c.sessid, c.Src())
			c.conn.Close()
			return
		}
		db.DLPrintf("NETSRV", "rep %v\n", fcall)
		var writableFcall np.WritableFcall
		if c.wireCompat {
			writableFcall = fcall.ToWireCompatible()
		} else {
			writableFcall = fcall
		}
		if err := npcodec.MarshalFcall(writableFcall, c.bw); err != nil {
			db.DLPrintf("NETSRV_ERR", "%v writer %v err %v\n", c.sessid, c.Src(), err)
			continue
		}
		if error := c.bw.Flush(); error != nil {
			db.DLPrintf("NETSRV_ERR", "flush %v to %v err %v", fcall, c.Src(), error)
		}
	}
}
