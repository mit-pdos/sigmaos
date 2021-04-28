package npsrv

import (
	"bufio"
	"io"
	"log"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npclnt"
	"ulambda/npcodec"
)

const (
	Msglen = 64 * 1024
)

type Channel struct {
	npc     NpConn
	conn    net.Conn
	np      NpAPI
	br      *bufio.Reader
	bw      *bufio.Writer
	replies chan *np.Fcall
	closed  bool
}

type RelayChannel struct {
	c      *Channel
	relay  *npclnt.NpChan
	isTail bool
}

func MakeChannel(npc NpConn, conn net.Conn) *Channel {
	npapi := npc.Connect(conn)
	c := &Channel{npc,
		conn,
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

func MakeRelayChannel(npc NpConn, conn net.Conn, relay *npclnt.NpChan, isTail bool) *RelayChannel {
	npapi := npc.Connect(conn)
	c := &Channel{npc,
		conn,
		npapi,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
	}
	rc := &RelayChannel{c, relay, isTail}
	go rc.writer()
	go rc.reader()
	return rc
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
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Print("Serve: unmarshal error: ", err)
		} else {
			db.DLPrintf("9PCHAN", "Reader sv req: %v\n", fcall)
			go c.serve(fcall)
		}
	}
}

func (c *Channel) close() {
	db.DLPrintf("9PCHAN", "Close: %v", c.conn.RemoteAddr())
	c.closed = true
	close(c.replies)
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
		frame, err := npcodec.Marshal(fcall)
		if err != nil {
			log.Print("Writer: marshal error: ", err)
		} else {
			// log.Print("Srv: Rframe ", len(frame), frame)
			err = npcodec.WriteFrame(c.bw, frame)
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

func (rc *RelayChannel) reader() {
	db.DLPrintf("9PCHAN", "Reader conn from %v\n", rc.c.Src())
	for {
		frame, err := npcodec.ReadFrame(rc.c.br)
		if err != nil {
			db.DLPrintf("9PCHAN", "Peer %v closed/erred %v\n", rc.c.Src(), err)
			if err == io.EOF {
				rc.c.close()
			}
			return
		}
		// XXX This work can and should be done in another thread...
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Print("Serve: unmarshal error: ", err)
		} else {
			// Only call down the chain if we aren't at the tail.
			if !rc.isTail {
				// XXX This definitely is *not* the most efficient way to do this...
				rc.relay.Call(fcall.Msg)
			}
			db.DLPrintf("9PCHAN", "Reader sv req: %v\n", fcall)
			rc.serve(fcall)
		}
	}
}

func (rc *RelayChannel) serve(fc *np.Fcall) {
	t := fc.Tag
	reply, rerror := rc.c.dispatch(fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fcall := &np.Fcall{}
	fcall.Type = reply.Type()
	fcall.Msg = reply
	fcall.Tag = t
	if !rc.c.closed {
		rc.c.replies <- fcall
	}
}

func (rc *RelayChannel) writer() {
	for {
		fcall, ok := <-rc.c.replies
		if !ok {
			return
		}
		db.DLPrintf("9PCHAN", "Writer rep: %v\n", fcall)
		frame, err := npcodec.Marshal(fcall)
		if err != nil {
			log.Print("Writer: marshal error: ", err)
		} else {
			// log.Print("Srv: Rframe ", len(frame), frame)
			err = npcodec.WriteFrame(rc.c.bw, frame)
			if err != nil {
				log.Print("Writer: WriteFrame error ", err)
				return
			}
			err = rc.c.bw.Flush()
			if err != nil {
				log.Print("Writer: Flush error ", err)
				return
			}
		}
	}
}
