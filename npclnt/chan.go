package npclnt

import (
	"bufio"
	"fmt"
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
	addrs       []string
	closed      bool
	br          *bufio.Reader
	bw          *bufio.Writer
	requests    chan *RpcT
	nextTag     np.Ttag
	outstanding map[np.Ttag]*RpcT
}

func mkChan(addrs []string) (*Chan, error) {
	db.DLPrintf("9PCHAN", "mkChan to %v\n", addrs)
	ch := &Chan{}
	ch.requests = make(chan *RpcT)
	ch.outstanding = make(map[np.Ttag]*RpcT)
	ch.addrs = addrs
	ch.connect()
	go ch.writer()
	go ch.reader()

	return ch, nil
}

func (ch *Chan) Dst() string {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	return ch.dstL()
}

func (ch *Chan) dstL() string {
	return ch.conn.RemoteAddr().String()
}

func (ch *Chan) Src() string {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	return ch.conn.LocalAddr().String()
}

func (ch *Chan) getOutstanding() map[np.Ttag]*RpcT {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	cp := make(map[np.Ttag]*RpcT)
	for t, r := range ch.outstanding {
		cp[t] = r
	}
	return cp
}

func (ch *Chan) terminateOutstanding() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.terminateOutstandingL()
}

func (ch *Chan) terminateOutstandingL() {
	for _, rpc := range ch.outstanding {
		close(rpc.replych)
	}
}

func (ch *Chan) resendOutstanding() {
	outstanding := ch.getOutstanding()
	for t, r := range outstanding {
		// Retry sending the request in a separate thread
		go func(t np.Ttag, r *RpcT) {
			ch.lookupDel(t)
			ch.requests <- r
		}(t, r)
	}
}

func (ch *Chan) Close() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	db.DLPrintf("9PCHAN", "Close chan to %v\n", ch.dstL())
	ch.terminateOutstandingL()
	close(ch.requests)
	ch.closed = true
	ch.conn.Close()
}

func (ch *Chan) resetConnection() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.br = nil
	ch.bw = nil
	ch.connectL()
}

func (ch *Chan) getBw() (*bufio.Writer, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.conn == nil || ch.bw == nil {
		err := ch.connectL()
		if err != nil {
			return nil, err
		}
	}
	return ch.bw, nil
}

func (ch *Chan) getBr() (*bufio.Reader, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.conn == nil || ch.br == nil {
		err := ch.connectL()
		if err != nil {
			return nil, err
		}
	}
	return ch.br, nil
}

func (ch *Chan) connect() error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	return ch.connectL()
}

func (ch *Chan) connectL() error {
	for _, addr := range ch.addrs {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			db.DLPrintf("9PCHAN", "Connect to %v err %v\n", addr, err)
			continue
		}
		db.DLPrintf("9PCHAN", "Connect to %v from %v\n", addr, c.LocalAddr())
		ch.conn = c
		ch.br = bufio.NewReaderSize(c, Msglen)
		ch.bw = bufio.NewWriterSize(c, Msglen)
		return nil
	}
	db.DLPrintf("9PCHAN", "No successful connections %v\n", ch.addrs)
	return fmt.Errorf("Unable to connect")
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
		db.DLPrintf("9PCHAN", "Error reply ch closed %v -> %v\n", ch.Src(), ch.Dst())
		return nil, io.EOF
	}
	db.DLPrintf("9PCHAN", "RPC reply %v %v\n", reply.fc, reply.err)
	return reply.fc, reply.err
}

func (ch *Chan) writer() {
	bw, err := ch.getBw()
	if err != nil {
		db.DLPrintf("9PCHAN", "Writer: no viable connections: %v", err)
		ch.Close()
		return
	}
	for {
		rpc, ok := <-ch.requests
		if !ok {
			return
		}
		t := ch.allocate(rpc)
		rpc.req.Tag = t
		db.DLPrintf("9PCHAN", "Writer: to %v %v\n", ch.Dst(), rpc.req)
		err = npcodec.MarshalFcallToWriter(rpc.req, bw)
		if err != nil {
			if strings.Contains(err.Error(), "marshal error") {
				rpc.replych <- &Reply{nil, err}
			}
			// Retry sends on network error
			if strings.Contains(err.Error(), "EOF") {
				ch.resetConnection()
				// Get the bw for the latest connection
				bw, err = ch.getBw()
				// If none was available, close the channel.
				if err != nil {
					db.DLPrintf("9PCHAN", "Writer: no viable connections: %v", err)
					ch.Close()
					return
				}

				ch.resendOutstanding()
				continue
			}
			// If exit the thread if the connection is broken
			if strings.Contains(err.Error(), "WriteFrame error") {
				// XXX network errors here too?
				log.Fatal(err)
				return
			}
		} else {
			err = ch.bw.Flush()
			// XXX Network errors here too?
			if err != nil {
				log.Fatalf("Flush error %v\n", err)
				return
			}
		}
	}
}

func (ch *Chan) reader() {
	// Get the br for the latest connection
	br, err := ch.getBr()
	// If none was available, close the channel.
	if err != nil {
		db.DLPrintf("9PCHAN", "Reader: no viable connections: %v", err)
		ch.Close()
		return
	}
	for {
		db.DLPrintf("9PCHAN", "Reader: about to ReadFrame %v\n", ch.Dst())
		frame, err := npcodec.ReadFrame(br)
		// On connection error, retry
		if err == io.EOF || (err != nil && strings.Contains(err.Error(), "connection reset by peer")) {
			ch.resetConnection()
			// Get the br for the latest connection
			br, err = ch.getBr()
			// If none was available, close the channel.
			if err != nil {
				db.DLPrintf("9PCHAN", "Reader: no viable connections: %v", err)
				ch.Close()
				return
			}
			continue
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
