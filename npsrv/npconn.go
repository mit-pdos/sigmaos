package npsrv

import (
	"bufio"
	"io"
	"log"
	"net"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

const (
	Msglen = 64 * 1024
)

type Channel struct {
	mu         sync.Mutex
	npc        NpConn
	conn       net.Conn
	wireCompat bool
	np         NpAPI
	br         *bufio.Reader
	bw         *bufio.Writer
	replies    chan *np.Fcall
	closed     bool
}

func MakeChannel(npc NpConn, conn net.Conn, wireCompat bool) *Channel {
	npapi := npc.Connect(conn)
	c := &Channel{sync.Mutex{},
		npc,
		conn,
		wireCompat,
		npapi,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
	}
	go c.writer()
	go c.reader()
	return c
}

func (c *Channel) Src() string {
	return c.conn.RemoteAddr().String()
}

func (c *Channel) Dst() string {
	return c.conn.LocalAddr().String()
}

func (c *Channel) dispatch(msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	switch req := msg.(type) {
	case np.Tversion:
		reply := &np.Rversion{}
		err := c.np.Version(req, reply)
		return *reply, err
	case np.Tauth:
		reply := &np.Rauth{}
		err := c.np.Auth(req, reply)
		return *reply, err
	case np.Tattach:
		reply := &np.Rattach{}
		err := c.np.Attach(req, reply)
		return *reply, err
	case np.Tflush:
		reply := &np.Rflush{}
		err := c.np.Flush(req, reply)
		return *reply, err
	case np.Twalk:
		reply := &np.Rwalk{}
		err := c.np.Walk(req, reply)
		return *reply, err
	case np.Topen:
		reply := &np.Ropen{}
		err := c.np.Open(req, reply)
		return *reply, err
	case np.Twatchv:
		reply := &np.Ropen{}
		err := c.np.WatchV(req, reply)
		return *reply, err
	case np.Tcreate:
		reply := &np.Rcreate{}
		err := c.np.Create(req, reply)
		return *reply, err
	case np.Tread:
		reply := &np.Rread{}
		err := c.np.Read(req, reply)
		return *reply, err
	case np.Treadv:
		reply := &np.Rread{}
		err := c.np.ReadV(req, reply)
		return *reply, err
	case np.Twrite:
		reply := &np.Rwrite{}
		err := c.np.Write(req, reply)
		return *reply, err
	case np.Twritev:
		reply := &np.Rwrite{}
		err := c.np.WriteV(req, reply)
		return *reply, err
	case np.Tclunk:
		reply := &np.Rclunk{}
		err := c.np.Clunk(req, reply)
		return *reply, err
	case np.Tremove:
		reply := &np.Rremove{}
		err := c.np.Remove(req, reply)
		return *reply, err
	case np.Tstat:
		reply := &np.Rstat{}
		err := c.np.Stat(req, reply)
		return *reply, err
	case np.Twstat:
		reply := &np.Rwstat{}
		err := c.np.Wstat(req, reply)
		return *reply, err
	case np.Trenameat:
		reply := &np.Rrenameat{}
		err := c.np.Renameat(req, reply)
		return *reply, err
	default:
		return np.ErrUnknownMsg, nil
	}
}

func (c *Channel) reader() {
	db.DLPrintf("9PCHAN", "Reader conn from %v\n", c.Src())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			db.DLPrintf("9PCHAN", "Peer %v closed/erred %v\n", c.Src(), err)
			if err == io.EOF {
				c.close()
			}
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
			log.Print("Serve: unmarshal error: ", err)
		} else {
			db.DLPrintf("9PCHAN", "Reader sv req: %v\n", fcall)
			go c.serve(fcall)
		}
	}
}

func (c *Channel) close() {
	db.DLPrintf("9PCHAN", "Close: %v", c.conn.RemoteAddr())
	c.mu.Lock()
	c.closed = true
	close(c.replies)
	c.mu.Unlock()
	c.np.Detach()
}

func (c *Channel) serve(fc *np.Fcall) {
	t := fc.Tag
	reply, rerror := c.dispatch(fc.Msg)
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

func (c *Channel) writer() {
	for {
		fcall, ok := <-c.replies
		if !ok {
			return
		}
		db.DLPrintf("9PCHAN", "Writer rep: %v\n", fcall)
		var frame []byte
		var err error
		if c.wireCompat {
			fcallWC := fcall.ToWireCompatible()
			frame, err = npcodec.Marshal(fcallWC)
		} else {
			frame, err = npcodec.Marshal(fcall)
		}
		if err != nil {
			log.Print("Writer: marshal error: ", err)
		} else {
			sendBuf := false
			var data []byte
			switch fcall.Type {
			case np.TTwrite:
				msg := fcall.Msg.(np.Twrite)
				data = msg.Data
				sendBuf = true
			case np.TTwritev:
				msg := fcall.Msg.(np.Twritev)
				data = msg.Data
				sendBuf = true
			case np.TRread:
				msg := fcall.Msg.(np.Rread)
				data = msg.Data
				sendBuf = true
			default:
			}
			if sendBuf {
				err = npcodec.WriteFrameAndBuf(c.bw, frame, data)
			} else {
				err = npcodec.WriteFrame(c.bw, frame)
			}
			if err != nil {
				log.Print("Writer: WriteFrame error ", err)
				return
			}
			err = c.bw.Flush()
			if err != nil {
				log.Print("Writer: Flush error ", err)
				return
			}
		}
	}
}
