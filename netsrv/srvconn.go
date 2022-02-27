package netsrv

import (
	"bufio"
	"log"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/proc"
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
	db.DLPrintf("NETSRV", "Reader conn from %v\n", c.Src())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			db.DLPrintf("NETSRV", "Peer %v closed/erred %v\n", c.Src(), err)

			// If the sessid hasn't been set, we haven't received any valid ops yet,
			// so the session has not been added to the session table. If this is the
			// case, don't close the session (there is nothing to close).
			if c.sessid != 0 {
				// Set up the detach fcall
				dFcall := np.MakeFcall(np.Tdetach{}, c.sessid, nil)
				// Detach the session to remove ephemeral files and close open fids.
				// Set replies to nil to indicate that we don't need a response.
				c.protsrv.Process(dFcall, nil)
				c.protsrv.CloseSession(c.sessid, c.replies)
			}

			// close the reply channel, so that conn writer() terminates
			db.DLPrintf("NETSRV", "Reader: close replies for %v\n", c.Src())
			close(c.replies)
			return
		}
		var fcall *np.Fcall
		if c.wireCompat {
			fcallWC := &np.FcallWireCompat{}
			err = npcodec.Unmarshal(frame, fcallWC)
			fcall = fcallWC.ToInternal()
		} else {
			fcall = &np.Fcall{}
			err = npcodec.Unmarshal(frame, fcall)
		}
		if err != nil {
			log.Print("%v: reader: bad fcall: ", proc.GetName(), err)
		} else {
			db.DLPrintf("NETSRV", "srv req %v\n", fcall)
			if c.sessid == 0 {
				c.sessid = fcall.Session
			} else if c.sessid != fcall.Session {
				log.Fatal("reader: two sess (%v and %v) on conn?\n", c.sessid, fcall.Session)
			}
			c.protsrv.Process(fcall, c.replies)
		}
	}
}

func (c *SrvConn) writer() {
	for {
		fcall, ok := <-c.replies
		if !ok {
			db.DLPrintf("NETSRV", "writer: close conn from %v\n", c.Src())
			c.conn.Close()
			return
		}
		db.DLPrintf("NETSRV", "srv rep %v\n", fcall)
		var err *np.Err
		var writableFcall np.WritableFcall
		if c.wireCompat {
			writableFcall = fcall.ToWireCompatible()
		} else {
			writableFcall = fcall
		}
		err = npcodec.MarshalFcallToWriter(writableFcall, c.bw)
		if err != nil {
			log.Printf("%v: writer err %v\n", proc.GetProgram(), err)
		} else {
			error := c.bw.Flush()
			if error != nil {
				log.Printf("%v: flush %v err %v", proc.GetProgram(), fcall, err)
			}
		}
	}
}
