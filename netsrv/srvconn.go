package netsrv

import (
	"bufio"
	"log"
	"net"
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
	fssrv      protsrv.FsServer
	conn       net.Conn
	wireCompat bool
	np         protsrv.Protsrv
	br         *bufio.Reader
	bw         *bufio.Writer
	replies    chan *np.Fcall
	closed     bool
	sessions   map[np.Tsession]bool
}

func MakeSrvConn(srv *NetServer, conn net.Conn) *SrvConn {
	protsrv := srv.fssrv.Connect()
	c := &SrvConn{sync.Mutex{},
		srv.fssrv,
		conn,
		srv.wireCompat,
		protsrv,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
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
			go c.serve(fcall)
		}
	}
}

func (c *SrvConn) close() {
	db.DLPrintf("9PCHAN", "Close: %v", c.conn.RemoteAddr())
	c.mu.Lock()
	close(c.replies)
	if !c.closed {
		// Detach each session which used this channel
		for sess, _ := range c.sessions {
			c.np.Detach(sess)
		}
	}
	c.closed = true
	c.mu.Unlock()
}

// Remember which sessions we're handling to detach them when this channel
// closes.
func (c *SrvConn) registerSession(sess np.Tsession) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.sessions[sess]; !ok {
		c.sessions[sess] = true
		c.fssrv.SessionTable().RegisterSession(sess)
	}
}

func (c *SrvConn) serve(fc *np.Fcall) {
	t := fc.Tag
	c.registerSession(fc.Session)
	reply, rerror := protsrv.Dispatch(c.np, fc.Session, fc.Msg)
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
				log.Print("Writer: Flush error ", err)
				return
			}
		}
	}
}
