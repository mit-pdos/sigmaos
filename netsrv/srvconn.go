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
	db.DLPrintf("9PCHAN", "Reader conn from %v\n", c.Src())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			db.DLPrintf("9PCHAN", "Peer %v closed/erred %v\n", c.Src(), err)
			c.protsrv.CloseSession(c.sessid, c.replies)
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
			log.Print("reader: unmarshal error: ", err, fcall)
		} else {
			db.DLPrintf("9PCHAN", "Reader sv req: %v\n", fcall)
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
			return
		}
		db.DLPrintf("9PCHAN", "Writer rep: %v\n", fcall)
		var err error
		var writableFcall np.WritableFcall
		if c.wireCompat {
			writableFcall = fcall.ToWireCompatible()
		} else {
			writableFcall = fcall
		}
		err = npcodec.MarshalFcallToWriter(writableFcall, c.bw)
		if err != nil {
			log.Printf("%v: writer err %v\n", db.GetName(), err)
		} else {
			err = c.bw.Flush()
			if err != nil {
				log.Printf("%v: flush %v %v err %v", db.GetName(), fcall, err)
			}
		}
	}
}
