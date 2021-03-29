package npclnt

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
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
	db.DLPrintf("9PCHAN", "mkChan to %v\n", addr)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		db.DLPrintf("9PCHAN", "mkChan to %v err %v\n", addr, err)
		return nil, err
	}
	db.DLPrintf("9PCHAN", "mkChan to %v from %v\n", addr, c.LocalAddr())
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

func (ch *Chan) Dst() string { return ch.conn.RemoteAddr().String() }

func (ch *Chan) Close() {
	db.DLPrintf("9PCHAN", "Close chan to %v\n", ch.Dst())
	ch.mu.Lock()
	for _, rpc := range ch.outstanding {
		close(rpc.replych)
	}
	close(ch.requests)
	ch.closed = true
	ch.conn.Close()
	ch.mu.Unlock()
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
	db.DLPrintf("9PCHAN", "RPC %v to %v\n", fc, ch.Dst())
	ch.mu.Lock()
	closed := ch.closed
	ch.mu.Unlock()
	if closed {
		db.DLPrintf("9PCHAN", "Error ch to %v closed\n", ch.Dst())
		return nil, io.EOF
	}
	rpc := mkRpcT(fc)
	ch.requests <- rpc
	reply, ok := <-rpc.replych
	if !ok {
		db.DLPrintf("9PCHAN", "Error reply ch closed %v\n", ch.Dst())
		return nil, io.EOF
	}
	db.DLPrintf("9PCHAN", "RPC reply %v %v\n", reply.fc, reply.err)
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
		db.DLPrintf("9PCHAN", "Writer: to %v %v\n", ch.Dst(), rpc.req)
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
		db.DLPrintf("9PCHAN", "Reader: about to ReadFrame %v\n", ch.Dst())
		frame, err := npcodec.ReadFrame(ch.br)
		if err == io.EOF || (err != nil && strings.Contains(err.Error(), "connection reset by peer")) {
			ch.Close()
			return
		}
		if err != nil {
			db.DLPrintf("9PCHAN", "Reader: ReadFrame error %v\n", err)
			return
		}
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			db.DLPrintf("9PCHAN", "Reader: Unmarshal error %v\n", err)
		} else {
			rpc, ok := ch.lookupDel(fcall.Tag)
			if ok {
				db.DLPrintf("9PCHAN", "Reader: from %v %v\n", ch.Dst(), fcall)
				rpc.replych <- &Reply{fcall, nil}
			}
		}
	}
}
