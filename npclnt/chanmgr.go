package npclnt

import (
	"bufio"
	"net"
	"sync"

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
	mu    sync.Mutex
	conns map[string]*NpConn
}

func makeChanMgr() *ChanMgr {
	cm := &ChanMgr{}
	cm.conns = make(map[string]*NpConn)
	return cm
}

func (cm *ChanMgr) lookup(addr string) (*NpConn, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.conns[addr]
	return conn, ok
}

func (cm *ChanMgr) add(addr string, conn *NpConn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.conns[addr] = conn
}

func (cm *ChanMgr) Close(addr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.conns[addr]
	if ok {
		conn.conn.Close()
		delete(cm.conns, addr)
	}
}

func (cm *ChanMgr) makeCall(addr string, req np.Tmsg) (np.Tmsg, error) {
	conn, ok := cm.lookup(addr)
	if !ok {
		var err error
		c, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		conn = &NpConn{c,
			bufio.NewReaderSize(c, Msglen),
			bufio.NewWriterSize(c, Msglen)}
		cm.add(addr, conn)

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
