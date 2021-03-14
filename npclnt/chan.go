package npclnt

import (
	"bufio"
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
	req   *np.Fcall
	reply chan *Reply
}

func mkRpcT(fc *np.Fcall) *RpcT {
	rpc := &RpcT{}
	rpc.req = fc
	rpc.reply = make(chan *Reply)
	return rpc
}

type Chan struct {
	mu          sync.Mutex
	conn        net.Conn
	br          *bufio.Reader
	bw          *bufio.Writer
	requests    chan *RpcT
	nextTag     np.Ttag
	outstanding map[np.Ttag]*RpcT
}

func mkChan(addr string) (*Chan, error) {
	var err error
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
	db.DPrintf("Close chan: %v\n", ch.conn)

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
	rpc := mkRpcT(fc)
	ch.requests <- rpc
	reply := <-rpc.reply
	return reply.fc, reply.err
}

func (ch *Chan) writer() {
	for {
		rpc := <-ch.requests
		t := ch.allocate(rpc)
		rpc.req.Tag = t
		db.DPrintf("Writer: %v\n", rpc.req)
		frame, err := npcodec.Marshal(rpc.req)
		if err != nil {
			rpc.reply <- &Reply{nil, err}
		} else {
			err := npcodec.WriteFrame(ch.bw, frame)
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
		if err != nil {
			log.Printf("ReadFrame error %v\n", err)
			return
		}
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Printf("Unmarshal error %v\n", err)
			return
		}
		rpc, ok := ch.lookupDel(fcall.Tag)
		if ok {
			db.DPrintf("reader: %v\n", fcall)
			rpc.reply <- &Reply{fcall, nil}
		}
	}
}
