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

func MakeSrvConn(conn net.Conn, fssrv protsrv.FsServer, wireCompat bool) *SrvConn {
	protsrv := fssrv.Connect()
	c := &SrvConn{sync.Mutex{},
		fssrv,
		conn,
		wireCompat,
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

func (c *SrvConn) dispatch(sess np.Tsession, msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	switch req := msg.(type) {
	case np.Tversion:
		reply := &np.Rversion{}
		err := c.np.Version(sess, req, reply)
		return *reply, err
	case np.Tauth:
		reply := &np.Rauth{}
		err := c.np.Auth(sess, req, reply)
		return *reply, err
	case np.Tattach:
		reply := &np.Rattach{}
		err := c.np.Attach(sess, req, reply)
		return *reply, err
	case np.Tflush:
		reply := &np.Rflush{}
		err := c.np.Flush(sess, req, reply)
		return *reply, err
	case np.Twalk:
		reply := &np.Rwalk{}
		err := c.np.Walk(sess, req, reply)
		return *reply, err
	case np.Topen:
		reply := &np.Ropen{}
		err := c.np.Open(sess, req, reply)
		return *reply, err
	case np.Twatchv:
		reply := &np.Ropen{}
		err := c.np.WatchV(sess, req, reply)
		return *reply, err
	case np.Tcreate:
		reply := &np.Rcreate{}
		err := c.np.Create(sess, req, reply)
		return *reply, err
	case np.Tread:
		reply := &np.Rread{}
		err := c.np.Read(sess, req, reply)
		return *reply, err
	case np.Twrite:
		reply := &np.Rwrite{}
		err := c.np.Write(sess, req, reply)
		return *reply, err
	case np.Tclunk:
		reply := &np.Rclunk{}
		err := c.np.Clunk(sess, req, reply)
		return *reply, err
	case np.Tremove:
		reply := &np.Rremove{}
		err := c.np.Remove(sess, req, reply)
		return *reply, err
	case np.Tremovefile:
		reply := &np.Rremove{}
		err := c.np.RemoveFile(sess, req, reply)
		return *reply, err
	case np.Tstat:
		reply := &np.Rstat{}
		err := c.np.Stat(sess, req, reply)
		return *reply, err
	case np.Twstat:
		reply := &np.Rwstat{}
		err := c.np.Wstat(sess, req, reply)
		return *reply, err
	case np.Trenameat:
		reply := &np.Rrenameat{}
		err := c.np.Renameat(sess, req, reply)
		return *reply, err
	case np.Tgetfile:
		reply := &np.Rgetfile{}
		err := c.np.GetFile(sess, req, reply)
		return *reply, err
	case np.Tsetfile:
		reply := &np.Rwrite{}
		err := c.np.SetFile(sess, req, reply)
		return *reply, err
	default:
		return np.ErrUnknownMsg, nil
	}
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

	c.sessions[sess] = true
}

func (c *SrvConn) serve(fc *np.Fcall) {
	t := fc.Tag
	c.registerSession(fc.Session)
	// XXX Avoid doing this every time
	c.fssrv.SessionTable().RegisterSession(fc.Session)
	reply, rerror := c.dispatch(fc.Session, fc.Msg)
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
