package npclnt

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type Reply struct {
	fc  *np.Fcall
	err error
}

type RpcT struct {
	req     *np.Fcall
	replych chan *Reply
}

func mkRpcT(fc *np.Fcall) *RpcT {
	rpc := &RpcT{}
	rpc.req = fc
	rpc.replych = make(chan *Reply)
	return rpc
}

type Chan struct {
	mu          sync.Mutex
	conn        net.Conn
	closed      bool
	br          *bufio.Reader
	bw          *bufio.Writer
	requests    chan *RpcT
	nextTag     np.Ttag
	outstanding map[np.Ttag]*RpcT
}

func mkChan(addr string) (*Chan, error) {
	var err error
	db.DPrintf("mkChan to %v\n", addr)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	ch := &Chan{}
	ch.conn = c
	ch.br = bufio.NewReaderSize(c, Msglen)
	ch.bw = bufio.NewWriterSize(c, Msglen)
	ch.requests = make(chan *RpcT)
	ch.outstanding = make(map[np.Ttag]*RpcT)
	go ch.writer()
	go ch.reader()

	return ch, nil
}

func (ch *Chan) Close() {
	db.DPrintf("Close chan: %v->%v\n", ch.conn.LocalAddr(), ch.conn.RemoteAddr())
	for _, rpc := range ch.outstanding {
		close(rpc.replych)
	}
	close(ch.requests)
	ch.closed = true
	ch.conn.Close()
}

func (ch *Chan) allocate(req *RpcT) np.Ttag {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	t := ch.nextTag
	ch.nextTag += 1
	ch.outstanding[t] = req
	return t
}

func (ch *Chan) lookupDel(t np.Ttag) (*RpcT, bool) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	rpc, ok := ch.outstanding[t]
	delete(ch.outstanding, t)
	return rpc, ok
}

func (ch *Chan) RPC(fc *np.Fcall) (*np.Fcall, error) {
	db.DPrintf("RPC on ch: %v->%v\n", ch.conn.LocalAddr(), ch.conn.RemoteAddr())
	if ch.closed {
		log.Fatalf("ch closed: %v %v\n", ch.conn.LocalAddr(), ch.conn.RemoteAddr())
	}
	rpc := mkRpcT(fc)
	ch.requests <- rpc
	reply, ok := <-rpc.replych
	if !ok {
		return nil, fmt.Errorf("Channel closed")
	}
	return reply.fc, reply.err
}

func (ch *Chan) writer() {
	for {
		rpc, ok := <-ch.requests
		if !ok {
			return
		}
		t := ch.allocate(rpc)
		rpc.req.Tag = t
		db.DPrintf("Writer: %v\n", rpc.req)
		frame, err := npcodec.Marshal(rpc.req)
		if err != nil {
			rpc.replych <- &Reply{nil, err}
		} else {
			err := npcodec.WriteFrame(ch.bw, frame)
			if err == io.EOF {
				ch.Close()
				return
			}
			if err != nil {
				log.Fatalf("WriteFrame error %v\n", err)
				return
			}
			err = ch.bw.Flush()
			if err != nil {
				log.Fatalf("Flush error %v\n", err)
				return
			}
		}
	}
}

func (ch *Chan) reader() {
	for {
		frame, err := npcodec.ReadFrame(ch.br)
		if err == io.EOF {
			ch.Close()
			return
		}
		if err != nil {
			log.Printf("reader: ReadFrame error %v\n", err)
			return
		}
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Printf("reader: Unmarshal error %v\n", err)
		} else {
			rpc, ok := ch.lookupDel(fcall.Tag)
			if ok {
				db.DPrintf("reader: %v\n", fcall)
				rpc.replych <- &Reply{fcall, nil}
			}
		}
	}
}
