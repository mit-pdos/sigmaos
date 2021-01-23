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
	npc  NpConn
	conn net.Conn
	np   NpAPI
	br   *bufio.Reader
	bw   *bufio.Writer
}

func MakeChannel(npc NpConn, conn net.Conn) *Channel {
	np := npc.Connect(conn)
	c := &Channel{npc,
		conn,
		np,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
	}
	return c
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
	case np.Tmkpipe:
		reply := &np.Rmkpipe{}
		err := c.np.Pipe(req, reply)
		return *reply, err
	default:
		return np.ErrUnknownMsg, nil
	}
}

func (c *Channel) Serve() {
	db.DPrintf("Server conn %v\n", c.conn.RemoteAddr())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			if err != io.EOF {
				log.Print("Serve: readFrame error: ", err)
			}
			return
		}
		fcall := &np.Fcall{}
		// log.Print("Tframe ", len(frame), frame)
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Print("Serve: unmarshal error: ", err)
			return
		}
		db.DPrintf("Srv: %v\n", fcall)
		// XXX start go routine
		reply, rerror := c.dispatch(fcall.Msg)
		if rerror != nil {
			reply = *rerror
		}
		fcall.Type = reply.Type()
		fcall.Msg = reply
		db.DPrintf("Srv: %v\n", fcall)
		frame, err = npcodec.Marshal(fcall)
		if err != nil {
			log.Print("Serve: marshal error: ", err)
			return
		}
		// log.Print("Srv: Rframe ", len(frame), frame)
		err = npcodec.WriteFrame(c.bw, frame)
		if err != nil {
			log.Print("Serve: WriteFrame error ", err)
			return
		}
		c.bw.Flush()
	}
}
