package replchain

import (
	"bufio"
	"io"
	"net"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/npcodec"
	"ulambda/protsrv"
)

// A connection between replicas
type RelayNetConn struct {
	mu     sync.Mutex
	rMu    sync.Mutex
	wMu    sync.Mutex
	conn   net.Conn
	np     protsrv.Protsrv
	br     *bufio.Reader
	bw     *bufio.Writer
	dst    string
	closed bool
}

func MakeRelayNetConn(addr string) (*RelayNetConn, error) {
	var err error
	db.DPrintf("RTCP", "mkChan to %v\n", addr)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		db.DPrintf("RTCP", "mkChan to %v err %v\n", addr, err)
		return nil, err
	}
	db.DPrintf("RTCP", "mkChan to %v from %v\n", addr, c.LocalAddr())
	rc := &RelayNetConn{}
	rc.conn = c
	rc.dst = addr
	rc.br = bufio.NewReaderSize(c, Msglen)
	rc.bw = bufio.NewWriterSize(c, Msglen)

	return rc, nil
}

func (rc *RelayNetConn) Send(frame []byte) error {
	rc.wMu.Lock()
	defer rc.wMu.Unlock()
	err := npcodec.WriteFrame(rc.bw, frame)
	if err == io.EOF {
		rc.Close()
		return err
	}
	if err != nil {
		db.DPrintf("RTCP", "WriteFrame error %v\n", err)
		return err
	}
	error := rc.bw.Flush()
	if error != nil {
		db.DPrintf("RTCP", "Flush error %v\n", err)
		return err
	}
	return nil
}

func (rc *RelayNetConn) Recv() ([]byte, error) {
	rc.rMu.Lock()
	defer rc.rMu.Unlock()
	if rc.isClosed() {
		return []byte{}, io.EOF
	}
	frame, err := npcodec.ReadFrame(rc.br)
	if err == io.EOF || (err != nil && (strings.Contains(err.Error(), "connection reset by peer") || strings.Contains(err.Error(), "use of closed network connection"))) {
		rc.Close()
		return nil, err
	}
	if err != nil {
		db.DPrintf("RTCP", "Reader: ReadFrame error %v\n", err)
		return nil, err
	}
	return frame, nil
}

func (rc *RelayNetConn) closeL() {
	db.DPrintf("RTCP", "Close relay chan to %v\n", rc.dst)
	rc.closed = true
	rc.conn.Close()
}

func (rc *RelayNetConn) Close() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.closeL()
}

func (rc *RelayNetConn) isClosed() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.closed
}
