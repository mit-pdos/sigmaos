package npsrv

import (
	"bufio"
	"log"
	"net"

	np "ulambda/ninep"
	"ulambda/npcodec"
)

const (
	Msglen = 64 * 1024
)

type Channel struct {
	npd  NpConn
	conn net.Conn
	fsc  NpAPI
	br   *bufio.Reader
	bw   *bufio.Writer
}

func MakeChannel(npd NpConn, conn net.Conn) *Channel {
	fsc := npd.Connect(conn)
	c := &Channel{npd,
		conn,
		fsc,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen)}
	return c
}

func (c *Channel) dispatch(msg np.Tmsg) (np.Tmsg, *np.Rerror) {
	switch req := msg.(type) {
	case np.Tversion:
		reply := &np.Rversion{}
		err := c.fsc.Version(req, reply)
		return *reply, err
	case np.Tauth:
		reply := &np.Rauth{}
		err := c.fsc.Auth(req, reply)
		return *reply, err
	case np.Tattach:
		reply := &np.Rattach{}
		err := c.fsc.Attach(req, reply)
		return *reply, err
	case np.Tflush:
		reply := &np.Rflush{}
		err := c.fsc.Flush(req, reply)
		return *reply, err
	case np.Twalk:
		reply := &np.Rwalk{}
		err := c.fsc.Walk(req, reply)
		return *reply, err
	case np.Topen:
		reply := &np.Ropen{}
		err := c.fsc.Open(req, reply)
		return *reply, err
	case np.Tcreate:
		reply := &np.Rcreate{}
		err := c.fsc.Create(req, reply)
		return *reply, err
	case np.Tread:
		reply := &np.Rread{}
		err := c.fsc.Read(req, reply)
		return *reply, err
	case np.Twrite:
		reply := &np.Rwrite{}
		err := c.fsc.Write(req, reply)
		return *reply, err
	case np.Tclunk:
		reply := &np.Rclunk{}
		err := c.fsc.Clunk(req, reply)
		return *reply, err
	case np.Tstat:
		reply := &np.Rstat{}
		err := c.fsc.Stat(req, reply)
		return *reply, err
	// case np.Tremove:
	// 	reply := &np.Rremove{}
	// 	err := c.fsc.Remove(req, reply)
	// 	return *reply, err
	// case np.Twstat:
	// 	reply := &np.Rwstat{}
	// 	err := c.fsc.Wstat(req, reply)
	// 	return *reply, err
	default:
		return np.ErrUnknownMsg, nil
	}
}

func (c *Channel) Serve() {
	log.Printf("Server conn %v\n", c.conn.RemoteAddr())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			log.Fatal("Server readMsg error: ", err)
		}
		fcall := &np.Fcall{}
		// log.Print("Tframe ", len(frame), frame)
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Fatal("Server unmarshal error: ", err)
		}
		log.Print(fcall)
		// XXX start go routine
		reply, rerror := c.dispatch(fcall.Msg)
		if rerror != nil {
			reply = *rerror
		}
		fcall.Type = reply.Type()
		fcall.Msg = reply
		log.Print(fcall)
		frame, err = npcodec.Marshal(fcall)
		if err != nil {
			log.Fatal("Server marshal error: ", err)
		}
		// log.Print("Rframe ", len(frame), frame)
		npcodec.WriteFrame(c.bw, frame)
		c.bw.Flush()
	}
}
