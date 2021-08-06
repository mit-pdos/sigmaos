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

func (ch *Chan) getOutstandingL() map[np.Ttag]*RpcT {
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
	ch.outstanding = map[np.Ttag]*RpcT{}
}

func (ch *Chan) resendOutstandingL() {
	outstanding := ch.getOutstandingL()
	db.DLPrintf("9PCHAN", "Resending outstanding requests: %v", outstanding)
	for t, r := range outstanding {
		// Retry sending the request in a separate thread
		go func(t np.Ttag, r *RpcT) {
			ch.mu.Lock()
			defer ch.mu.Unlock()

			if !ch.closed {
				ch.requests <- r
				db.DLPrintf("9PCHAN", "Resent outstanding request: %v", r)
			}
		}(t, r)
	}
}

func (ch *Chan) Close() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	db.DLPrintf("9PCHAN", "Close chan to %v\n", ch.dstL())
	ch.terminateOutstandingL()
	if !ch.closed {
		close(ch.requests)
	}
	ch.closed = true
	ch.conn.Close()
}

func (ch *Chan) resetConnection(br *bufio.Reader, bw *bufio.Writer) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	// If the expected buffered reader & writers have changed, then the connection
	// has already been reset by another thread. Avoid double-resetting.
	if br == ch.br && bw == ch.bw {
		db.DLPrintf("9PCHAN", "Resetting connection to %v\n", ch.dstL())
		ch.conn.Close()
		ch.br = nil
		ch.bw = nil
		ch.connectL()
		// Resend outstanding requests
		ch.resendOutstandingL()
		db.DLPrintf("9PCHAN", "Done resetting connection to %v\n", ch.dstL())
	}
}

func (ch *Chan) getBufio() (*bufio.Reader, *bufio.Writer, error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.conn == nil || (ch.br == nil || ch.bw == nil) {
		err := ch.connectL()
		if err != nil {
			return nil, nil, err
		}
	}
	return ch.br, ch.bw, nil
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
		ch.conn = c
		ch.br = bufio.NewReaderSize(c, Msglen)
		ch.bw = bufio.NewWriterSize(c, Msglen)
		db.DLPrintf("9PCHAN", "Connect %v -> %v bw:%p, br:%p\n", c.LocalAddr(), addr, ch.bw, ch.br)
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
	t := ch.allocate(rpc)
	rpc.req.Tag = t
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
	br, bw, err := ch.getBufio()
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
		// Get the bw for the latest connection
		br, bw, err = ch.getBufio()
		// If none was available, close the channel.
		if err != nil {
			db.DLPrintf("9PCHAN", "Writer: no viable connections: %v", err)
			ch.Close()
			return
		}
		db.DLPrintf("9PCHAN", "Writer: %v -> %v %v, %p\n", ch.Src(), ch.Dst(), rpc.req, bw)
		err = npcodec.MarshalFcallToWriter(rpc.req, bw)
		if err != nil {
			if strings.Contains(err.Error(), "marshal error") {
				ch.mu.Lock()
				if !ch.closed {
					rpc.replych <- &Reply{nil, err}
				}
				ch.mu.Unlock()
			}
			// Retry sends on network error
			if strings.Contains(err.Error(), "EOF") {
				db.DLPrintf("9PCHAN", "Writer: Connection error to %v\n", ch.Dst())
				ch.resetConnection(br, bw)
				continue
			}
			// If exit the thread if the connection is broken
			if strings.Contains(err.Error(), "WriteFrame error") {
				log.Fatal(err)
				return
			}
			db.DLPrintf("9PCHAN", "Writer: Connection error to %v: %v", ch.Dst(), err)
		} else {
			err = ch.bw.Flush()
			if err != nil {
				if strings.Contains(err.Error(), "connection reset by peer") {
					ch.resetConnection(br, bw)
				} else {
					log.Fatalf("Flush error %v\n", err)
					return
				}
			}
		}
	}
}

func (ch *Chan) reader() {
	// Get the br for the latest connection
	br, bw, err := ch.getBufio()
	// If none was available, close the channel.
	if err != nil {
		db.DLPrintf("9PCHAN", "Reader: no viable connections: %v", err)
		ch.Close()
		return
	}
	for {
		db.DLPrintf("9PCHAN", "Reader: about to ReadFrame from %v br:%p\n", ch.Dst(), br)
		frame, err := npcodec.ReadFrame(br)
		db.DLPrintf("9PCHAN", "Reader: ReadFrame from %v br:%p\n", ch.Dst(), br)
		// On connection error, retry
		if err == io.EOF || (err != nil && strings.Contains(err.Error(), "connection reset by peer")) {
			db.DLPrintf("9PCHAN", "Reader: Connection error to %v\n", ch.Dst())
			ch.resetConnection(br, bw)
			// Get the br for the latest connection
			br, bw, err = ch.getBufio()
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
				ch.mu.Lock()
				if !ch.closed {
					rpc.replych <- &Reply{fcall, nil}
				}
				ch.mu.Unlock()
			}
		}
	}
}
