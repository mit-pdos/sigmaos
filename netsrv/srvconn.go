package netsrv

import (
	"bufio"
	"log"
	"net"
	"runtime/debug"
	"strings"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/protsrv"
)

const (
	Msglen = 64 * 1024
)

type SrvConn struct {
	mu         sync.Mutex
	conn       net.Conn
	wireCompat bool
	br         *bufio.Reader
	bw         *bufio.Writer
	replies    chan *np.Fcall
	closed     bool
	protsrv    protsrv.FsServer
	sessions   map[np.Tsession]bool
}

func MakeSrvConn(srv *NetServer, conn net.Conn) *SrvConn {
	c := &SrvConn{sync.Mutex{},
		conn,
		srv.wireCompat,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
		srv.fssrv,
		make(map[np.Tsession]bool),
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
			c.close()
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
			c.sessions[fcall.Session] = true
			go c.serve(fcall)
		}
	}
}

func (c *SrvConn) close() {
	db.DLPrintf("9PCHAN", "Close: %v", c.conn.RemoteAddr())
	c.mu.Lock()

	close(c.replies)
	if !c.closed {
		for sid, _ := range c.sessions {
			c.protsrv.Detach(sid)
		}
	}
	c.closed = true
	c.mu.Unlock()
}

func (c *SrvConn) serve(fc *np.Fcall) {
	t := fc.Tag
	reply, rerror := c.protsrv.Dispatch(fc.Session, fc.Msg)
	if rerror != nil {
		reply = *rerror
	}

	fcall := &np.Fcall{}
	fcall.Type = reply.Type()
	fcall.Msg = reply
	fcall.Tag = t
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.replies <- fcall
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
			log.Print(err)
			// If exit the thread if the connection is broken
			if strings.Contains(err.Error(), "WriteFrame error") {
				return
			}
		} else {
			err = c.bw.Flush()
			if err != nil {
				stacktrace := debug.Stack()
				db.DLPrintf("NETSRV", "%v\nWriter: Flush error ", string(stacktrace), err)
				return
			}
		}
	}
}
