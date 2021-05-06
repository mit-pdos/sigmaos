package npsrv

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	db "ulambda/debug"
	//	np "ulambda/ninep"
	"ulambda/npcodec"
)

type RelayChan struct {
	mu     sync.Mutex
	conn   net.Conn
	np     NpAPI
	br     *bufio.Reader
	bw     *bufio.Writer
	dst    string
	closed bool
}

func MakeRelayChan(addr string) (*RelayChan, error) {
	var err error
	db.DLPrintf("9PCHAN", "mkChan to %v\n", addr)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		db.DLPrintf("9PCHAN", "mkChan to %v err %v\n", addr, err)
		return nil, err
	}
	db.DLPrintf("9PCHAN", "mkChan to %v from %v\n", addr, c.LocalAddr())
	rc := &RelayChan{}
	rc.conn = c
	rc.dst = addr
	rc.br = bufio.NewReaderSize(c, Msglen)
	rc.bw = bufio.NewWriterSize(c, Msglen)

	return rc, nil
}

func (rc *RelayChan) Send(frame []byte) error {
	err := npcodec.WriteFrame(rc.bw, frame)
	if err == io.EOF {
		rc.Close()
		return err
	}
	if err != nil {
		log.Fatalf("WriteFrame error %v\n", err)
		return err
	}
	err = rc.bw.Flush()
	if err != nil {
		log.Fatalf("Flush error %v\n", err)
		return err
	}
	return nil
}

func (rc *RelayChan) Recv() ([]byte, error) {
	frame, err := npcodec.ReadFrame(rc.br)
	if err == io.EOF || (err != nil && strings.Contains(err.Error(), "connection reset by peer")) {
		rc.Close()
		return nil, err
	}
	if err != nil {
		db.DLPrintf("9PCHAN", "Reader: ReadFrame error %v\n", err)
		log.Printf("ReadFrame error: %v", err)
		return nil, err
	}
	return frame, nil
}

func (rc *RelayChan) Close() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	db.DLPrintf("9PCHAN", "Close relay chan to %v\n", rc.dst)
	rc.closed = true
	rc.conn.Close()
}
