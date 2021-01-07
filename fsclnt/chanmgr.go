package fsclnt

import (
	"bufio"
	"log"
	"net"

	np "ulambda/ninep"
	"ulambda/npcodec"
)

// XXX duplicate
const (
	Msglen = 64 * 1024
)

type NpConn struct {
	conn net.Conn
	br   *bufio.Reader
	bw   *bufio.Writer
}

type ChanMgr struct {
	conns map[string]*NpConn
}

func makeChanMgr() *ChanMgr {
	cm := &ChanMgr{}
	cm.conns = make(map[string]*NpConn)
	return cm
}

func (cm *ChanMgr) makeCall(addr string, req np.Tmsg) (np.Tmsg, error) {
	conn, ok := cm.conns[addr]
	if !ok {
		var err error
		c, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		conn = &NpConn{c,
			bufio.NewReaderSize(c, Msglen),
			bufio.NewWriterSize(c, Msglen)}
		cm.conns[addr] = conn

	}
	fcall := &np.Fcall{}
	fcall.Type = req.Type()
	fcall.Msg = req
	log.Print(fcall)
	frame, err := npcodec.Marshal(fcall)
	if err != nil {
		log.Fatal("makeCall marshal error: ", err)
	}
	npcodec.WriteFrame(conn.bw, frame)
	conn.bw.Flush()

	frame, err = npcodec.ReadFrame(conn.br)
	if err != nil {
		log.Fatal("makeCall readMsg error: ", err)
	}
	fcall = &np.Fcall{}
	if err := npcodec.Unmarshal(frame, fcall); err != nil {
		log.Fatal("Server unmarshal error: ", err)
	}
	log.Print(fcall)
	return fcall.Msg, nil

}
