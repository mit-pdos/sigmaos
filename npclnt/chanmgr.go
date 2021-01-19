package npclnt

import (
	"bufio"
	"net"

	db "ulambda/debug"
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

func makeChanMgr(debug bool) *ChanMgr {
	cm := &ChanMgr{}
	cm.conns = make(map[string]*NpConn)
	db.SetDebug(debug)
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
	db.DPrintf("clnt: %v\n", fcall)
	frame, err := npcodec.Marshal(fcall)
	if err != nil {
		return nil, err
	}
	npcodec.WriteFrame(conn.bw, frame)
	conn.bw.Flush()

	frame, err = npcodec.ReadFrame(conn.br)
	if err != nil {
		return nil, err
	}
	fcall = &np.Fcall{}
	if err := npcodec.Unmarshal(frame, fcall); err != nil {
		return nil, err
	}
	db.DPrintf("clnt: %v\n", fcall)
	return fcall.Msg, nil

}
