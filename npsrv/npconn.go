package npsrv

import (
	"bufio"
	"io"
	"log"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
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
	name    string
}

func MakeChannel(npc NpConn, conn net.Conn, name string) *Channel {
	npapi := npc.Connect(conn)
	c := &Channel{npc,
		conn,
		npapi,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
		name,
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
	case np.Tcreate:
		reply := &np.Rcreate{}
		err := c.np.Create(req, reply)
		return *reply, err
	case np.Tread:
		reply := &np.Rread{}
		err := c.np.Read(req, reply)
		return *reply, err
	case np.Twrite:
		reply := &np.Rwrite{}
		err := c.np.Write(req, reply)
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
	default:
		return np.ErrUnknownMsg, nil
	}
}

func (c *Channel) reader() {
	db.DLPrintf(c.name, "9PCHAN", "Reader conn from %v\n", c.Src())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			if err == io.EOF {
				c.close()
			}
			return
		}
		fcall := &np.Fcall{}
		// log.Print("Tframe ", len(frame), frame)
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Print("Serve: unmarshal error: ", err)
		} else {
			db.DLPrintf(c.name, "9PCHAN", "Reader sv req: %v\n", fcall)
			go c.serve(fcall)
		}
	}
}

func (c *Channel) close() {
	db.DLPrintf(c.name, "9PCHAN", "Close: %v", c.conn.RemoteAddr())
	c.closed = true
	close(c.replies)
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
		db.DLPrintf(c.name, "9PCHAN", "Writer rep: %v\n", fcall)
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
