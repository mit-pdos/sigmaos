package netclnt

import (
	"bufio"
	"io"
	"log"
	"net"
	"runtime/debug"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/delay"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/proc"
)

const (
	MAX_TAG_ALLOC_RETRIES = 1000000
)

//
// Multiplexes RPCs onto a single network connection to server
//

// XXX duplicate
const (
	Msglen = 64 * 1024
)

type Reply struct {
	fc  *np.Fcall
	err *np.Err
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

type NetClnt struct {
	mu          sync.Mutex
	conn        net.Conn
	addrs       []string
	closed      bool
	br          *bufio.Reader
	bw          *bufio.Writer
	requests    chan *RpcT
	senders     sync.WaitGroup
	nextTag     np.Ttag
	outstanding map[np.Ttag]*RpcT
}

func MkNetClnt(addrs []string) (*NetClnt, *np.Err) {
	db.DLPrintf("NETCLNT", "mkNetClnt to %v\n", addrs)
	nc := &NetClnt{}
	nc.requests = make(chan *RpcT)
	nc.outstanding = make(map[np.Ttag]*RpcT)
	nc.addrs = addrs
	err := nc.connect()
	if err != nil {
		log.Printf("%v: mkNetClnt connect %v err %v\n", proc.GetProgram(), addrs, err)
		return nil, err
	}
	go nc.writer()
	go nc.reader()

	return nc, nil
}

func (nc *NetClnt) Dst() string {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	return nc.dstL()
}

func (nc *NetClnt) dstL() string {
	return nc.conn.RemoteAddr().String()
}

func (nc *NetClnt) Src() string {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	return nc.conn.LocalAddr().String()
}

func (nc *NetClnt) getOutstandingL() map[np.Ttag]*RpcT {
	cp := make(map[np.Ttag]*RpcT)
	for t, r := range nc.outstanding {
		cp[t] = r
	}
	return cp
}

func (nc *NetClnt) terminateOutstanding() {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	nc.terminateOutstandingL()
}

func (nc *NetClnt) terminateOutstandingL() {
	for _, rpc := range nc.outstanding {
		close(rpc.replych)
	}
	nc.outstanding = map[np.Ttag]*RpcT{}
}

func (nc *NetClnt) resendOutstandingL() {
	outstanding := nc.getOutstandingL()
	db.DLPrintf("NETCLNT", "Resending outstanding requests: %v", outstanding)
	for t, r := range outstanding {
		// Retry sending the request in a separate thread
		go func(t np.Ttag, r *RpcT) {
			nc.mu.Lock()
			defer nc.mu.Unlock()

			nc.senders.Add(1)
			defer nc.senders.Done()

			// XXX deadlocks, shouldn't hold lock
			if !nc.closed {
				nc.requests <- r
				db.DLPrintf("NETCLNT", "Resent outstanding request: %v", r)
			}
		}(t, r)
	}
}

func (nc *NetClnt) Close() {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	db.DLPrintf("NETCLNT", "Close conn to %v\n", nc.dstL())
	nc.terminateOutstandingL()
	wasClosed := nc.closed
	nc.closed = true

	// Close the requests channel asynchronously so we don't deadlock
	if !wasClosed {
		go func() {
			nc.senders.Wait()
			nc.conn.Close()
			close(nc.requests)
		}()
	}
}

func (nc *NetClnt) resetConnection(br *bufio.Reader, bw *bufio.Writer) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	// If the expected buffered reader & writers have changed, then the connection
	// has already been reset by another thread. Avoid double-resetting.
	if br == nc.br && bw == nc.bw {
		db.DLPrintf("NETCLNT", "Resetting connection to %v\n", nc.dstL())
		nc.conn.Close()
		nc.br = nil
		nc.bw = nil
		nc.connectL()
		// Resend outstanding requests
		nc.resendOutstandingL()
		db.DLPrintf("NETCLNT", "Done resetting connection to %v\n", nc.dstL())
	}
}

func (nc *NetClnt) getBufio() (*bufio.Reader, *bufio.Writer, *np.Err) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.conn == nil || (nc.br == nil || nc.bw == nil) {
		err := nc.connectL()
		if err != nil {
			return nil, nil, err
		}
	}
	return nc.br, nc.bw, nil
}

func (nc *NetClnt) connect() *np.Err {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	return nc.connectL()
}

func (nc *NetClnt) connectL() *np.Err {
	for _, addr := range nc.addrs {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			db.DLPrintf("NETCLNT", "NetClnt to %v err %v\n", addr, err)

			continue
		}
		nc.conn = c
		nc.br = bufio.NewReaderSize(c, Msglen)
		nc.bw = bufio.NewWriterSize(c, Msglen)
		db.DLPrintf("NETCLNT", "NetClnt %v -> %v bw:%p, br:%p\n", c.LocalAddr(), addr, nc.bw, nc.br)
		return nil
	}
	db.DLPrintf("NETCLNT", "No successful connections %v\n", nc.addrs)
	return np.MkErr(np.TErrEOF, "no connection")
}

func (nc *NetClnt) allocate(req *RpcT) np.Ttag {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	t := nc.nextTag
	nc.nextTag += 1
	retries := 0
	_, ok := nc.outstanding[t]
	// Retry until we get an unclaimed tag
	for ok {
		retries += 1
		if retries == MAX_TAG_ALLOC_RETRIES {
			debug.PrintStack()
			log.Fatalf("Error: Tried to allocate too many tags at once %v", len(nc.outstanding))
		}
		t = nc.nextTag
		nc.nextTag += 1
		_, ok = nc.outstanding[t]
	}
	nc.outstanding[t] = req
	return t
}

func (nc *NetClnt) lookupDel(t np.Ttag) (*RpcT, bool) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	rpc, ok := nc.outstanding[t]
	delete(nc.outstanding, t)
	return rpc, ok
}

func (nc *NetClnt) drainRequests() {
	ok := true
	for ok {
		_, ok = <-nc.requests
	}
}

func (nc *NetClnt) RPC(fc *np.Fcall) (*np.Fcall, *np.Err) {
	db.DLPrintf("RPC", "%v: req %v to %v\n", proc.GetName(), fc, nc.Dst())
	rpc := mkRpcT(fc)
	t := nc.allocate(rpc)
	rpc.req.Tag = t

	// maybe delay sending this RPC
	delay.MaybeDelayRPC()

	// Make sure the conn doesn't close under our feet by adding to the senders wg.
	nc.mu.Lock()
	closed := nc.closed
	if !closed {
		nc.senders.Add(1)
	}
	nc.mu.Unlock()

	if closed {
		db.DLPrintf("NETCLNT", "Error ch to %v closed\n", nc.Dst())
		return nil, np.MkErr(np.TErrEOF, nc.Dst())
	}
	nc.requests <- rpc

	nc.senders.Done()

	reply, ok := <-rpc.replych
	if !ok {
		db.DLPrintf("NETCLNT", "Error reply ch closed %v -> %v\n", nc.Src(), nc.Dst())
		return nil, np.MkErr(np.TErrEOF, nc.Dst())
	}
	db.DLPrintf("RPC", "%v: reply %v %v\n", proc.GetName(), reply.fc, reply.err)
	return reply.fc, reply.err
}

func (nc *NetClnt) writer() {
	// Need to make sure requests are drained so the requests channel can be safely closed
	defer nc.drainRequests()

	br, bw, err := nc.getBufio()
	if err != nil {
		db.DLPrintf("NETCLNT", "Writer: no viable connections: %v", err)
		nc.Close()
		return
	}
	for {
		rpc, ok := <-nc.requests
		if !ok {
			return
		}
		// Get the bw for the latest connection
		br, bw, err = nc.getBufio()
		// If none was available, close the conn
		if err != nil {
			db.DLPrintf("NETCLNT", "Writer: no viable connections: %v", err)
			nc.Close()
			return
		}
		err = npcodec.MarshalFcallToWriter(rpc.req, bw)
		if err != nil {
			if err.Code() == np.TErrBadFcall {
				nc.mu.Lock()
				if !nc.closed {
					rpc.replych <- &Reply{nil, err}
				}
				nc.mu.Unlock()
			}
			// Retry sends on network error
			// if np.IsErrEOF(err) {
			if err.Code() == np.TErrNet && err.Obj == "EOF" {
				db.DLPrintf("NETCLNT", "Writer: NetClntection error to %v\n", nc.Dst())
				nc.resetConnection(br, bw)
				continue
			}
			// If exit the thread if the connection is broken
			if err.Code() == np.TErrNet {
				log.Fatalf("FATAL MarshallFcallToWriter %v\n", err)
				return
			}
			db.DLPrintf("NETCLNT", "Writer: NetClnt error to %v: %v", nc.Dst(), err)
		} else {
			error := bw.Flush()
			if error != nil {
				if strings.Contains(error.Error(), "connection reset by peer") {
					nc.resetConnection(br, bw)
				} else {
					stacktrace := debug.Stack()
					db.DLPrintf("NETCLNT", "%v\nFlush error %v\n", string(stacktrace), err)
					return
				}
			}
		}
	}
}

func (nc *NetClnt) reader() {
	// Get the br for the latest connection
	br, bw, err := nc.getBufio()
	// If none was available, close the conn
	if err != nil {
		db.DLPrintf("NETCLNT", "Reader: no viable connections: %v", err)
		nc.Close()
		return
	}
	for {
		frame, err := npcodec.ReadFrame(br)
		// On connection error, retry
		// XXX write in terms of np.Err?
		if err == io.EOF || (err != nil && strings.Contains(err.Error(), "connection reset by peer")) {
			db.DLPrintf("NETCLNT", "Reader: NetClnt error to %v\n", nc.Dst())
			nc.resetConnection(br, bw)
			// Get the br for the latest connection
			var error error
			br, bw, error = nc.getBufio()
			// If none was available, close the conn
			if error != nil {
				db.DLPrintf("NETCLNT", "Reader: no viable connections: %v", error)
				nc.Close()
				return
			}
			continue
		}
		if err != nil {
			db.DLPrintf("NETCLNT", "Reader: ReadFrame error %v\n", err)
			nc.Close()
			return
		}
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			db.DLPrintf("NETCLNT", "Reader: Unmarshal error %v\n", err)
		} else {
			rpc, ok := nc.lookupDel(fcall.Tag)
			if ok {
				nc.mu.Lock()
				if !nc.closed {
					rpc.replych <- &Reply{fcall, nil}
				}
				nc.mu.Unlock()
			}
		}
	}
}
