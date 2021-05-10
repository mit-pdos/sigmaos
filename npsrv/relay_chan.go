package npsrv

import (
	//	"github.com/sasha-s/go-deadlock"

	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	db "ulambda/debug"
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
	db.DLPrintf("RCHAN", "mkChan to %v\n", addr)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		db.DLPrintf("RCHAN", "mkChan to %v err %v\n", addr, err)
		return nil, err
	}
	db.DLPrintf("RCHAN", "mkChan to %v from %v\n", addr, c.LocalAddr())
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
	rc.mu.Lock()
	defer rc.mu.Unlock()
	frame, err := npcodec.ReadFrame(rc.br)
	if err == io.EOF || (err != nil && strings.Contains(err.Error(), "connection reset by peer")) {
		rc.closeL()
		return nil, err
	}
	if err != nil {
		db.DLPrintf("RCHAN", "Reader: ReadFrame error %v\n", err)
		log.Printf("ReadFrame error: %v", err)
		return nil, err
	}
	return frame, nil
}

func (rc *RelayChan) closeL() {
	db.DLPrintf("RCHAN", "Close relay chan to %v\n", rc.dst)
	rc.closed = true
	rc.conn.Close()
}

func (rc *RelayChan) Close() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.closeL()
}
