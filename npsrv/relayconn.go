package npsrv

import (
	"bufio"
	"io"
	"net"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/npapi"
	"ulambda/npcodec"
)

// A connection between replicas
type RelayConn struct {
	mu     sync.Mutex
	rMu    sync.Mutex
	wMu    sync.Mutex
	conn   net.Conn
	np     npapi.NpAPI
	br     *bufio.Reader
	bw     *bufio.Writer
	dst    string
	closed bool
}

func MakeRelayConn(addr string) (*RelayConn, error) {
	var err error
	db.DLPrintf("RCHAN", "mkChan to %v\n", addr)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		db.DLPrintf("RCHAN", "mkChan to %v err %v\n", addr, err)
		return nil, err
	}
	db.DLPrintf("RCHAN", "mkChan to %v from %v\n", addr, c.LocalAddr())
	rc := &RelayConn{}
	rc.conn = c
	rc.dst = addr
	rc.br = bufio.NewReaderSize(c, Msglen)
	rc.bw = bufio.NewWriterSize(c, Msglen)

	return rc, nil
}

func (rc *RelayConn) Send(frame []byte) error {
	rc.wMu.Lock()
	defer rc.wMu.Unlock()
	err := npcodec.WriteFrame(rc.bw, frame)
	if err == io.EOF {
		rc.Close()
		return err
	}
	if err != nil {
		db.DLPrintf("RCHAN", "WriteFrame error %v\n", err)
		return err
	}
	err = rc.bw.Flush()
	if err != nil {
		db.DLPrintf("RCHAN", "Flush error %v\n", err)
		return err
	}
	return nil
}

func (rc *RelayConn) Recv() ([]byte, error) {
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
		db.DLPrintf("RCHAN", "Reader: ReadFrame error %v\n", err)
		return nil, err
	}
	return frame, nil
}

func (rc *RelayConn) closeL() {
	db.DLPrintf("RCHAN", "Close relay chan to %v\n", rc.dst)
	rc.closed = true
	rc.conn.Close()
}

func (rc *RelayConn) Close() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.closeL()
}

func (rc *RelayConn) isClosed() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.closed
}
