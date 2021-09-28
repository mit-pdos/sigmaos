package netclnt

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"runtime/debug"
	"strings"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

// XXX duplicate
const (
	Msglen = 64 * 1024
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

type NetClnt struct {
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

func MkNetClnt(addrs []string) (*NetClnt, error) {
	db.DLPrintf("NETCLNT", "mkNetClnt to %v\n", addrs)
	nc := &NetClnt{}
	nc.requests = make(chan *RpcT)
	nc.outstanding = make(map[np.Ttag]*RpcT)
	nc.addrs = addrs
	err := nc.connect()
	if err != nil {
		log.Printf("mkNetClnt: connect failed %v\n", err)
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
	if !nc.closed {
		close(nc.requests)
	}
	nc.closed = true
	nc.conn.Close()
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

func (nc *NetClnt) getBufio() (*bufio.Reader, *bufio.Writer, error) {
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

func (nc *NetClnt) connect() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	return nc.connectL()
}

func (nc *NetClnt) connectL() error {
	for _, addr := range nc.addrs {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			db.DLPrintf("NETCLNT", "NetClntect to %v err %v\n", addr, err)
			continue
		}
		nc.conn = c
		nc.br = bufio.NewReaderSize(c, Msglen)
		nc.bw = bufio.NewWriterSize(c, Msglen)
		db.DLPrintf("NETCLNT", "NetClntect %v -> %v bw:%p, br:%p\n", c.LocalAddr(), addr, nc.bw, nc.br)
		return nil
	}
	db.DLPrintf("NETCLNT", "No successful connections %v\n", nc.addrs)
	return fmt.Errorf("Unable to connect")
}

func (nc *NetClnt) allocate(req *RpcT) np.Ttag {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	t := nc.nextTag
	nc.nextTag += 1
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

func (nc *NetClnt) RPC(fc *np.Fcall) (*np.Fcall, error) {
	db.DLPrintf("NETCLNT", "RPC %v to %v\n", fc, nc.Dst())
	nc.mu.Lock()
	closed := nc.closed
	nc.mu.Unlock()
	if closed {
		db.DLPrintf("NETCLNT", "Error ch to %v closed\n", nc.Dst())
		return nil, io.EOF
	}
	rpc := mkRpcT(fc)
	t := nc.allocate(rpc)
	rpc.req.Tag = t
	nc.requests <- rpc
	reply, ok := <-rpc.replych
	if !ok {
		db.DLPrintf("NETCLNT", "Error reply ch closed %v -> %v\n", nc.Src(), nc.Dst())
		return nil, io.EOF
	}
	db.DLPrintf("NETCLNT", "RPC reply %v %v\n", reply.fc, reply.err)
	return reply.fc, reply.err
}

func (nc *NetClnt) writer() {
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
		db.DLPrintf("NETCLNT", "Writer: %v -> %v %v, %p\n", nc.Src(), nc.Dst(), rpc.req, bw)
		err = npcodec.MarshalFcallToWriter(rpc.req, bw)
		if err != nil {
			if strings.Contains(err.Error(), "marshal error") {
				nc.mu.Lock()
				if !nc.closed {
					rpc.replych <- &Reply{nil, err}
				}
				nc.mu.Unlock()
			}
			// Retry sends on network error
			if strings.Contains(err.Error(), "EOF") {
				db.DLPrintf("NETCLNT", "Writer: NetClntection error to %v\n", nc.Dst())
				nc.resetConnection(br, bw)
				continue
			}
			// If exit the thread if the connection is broken
			if strings.Contains(err.Error(), "WriteFrame error") {
				log.Fatal(err)
				return
			}
			db.DLPrintf("NETCLNT", "Writer: NetClntection error to %v: %v", nc.Dst(), err)
		} else {
			err = bw.Flush()
			if err != nil {
				if strings.Contains(err.Error(), "connection reset by peer") {
					nc.resetConnection(br, bw)
				} else {
					debug.PrintStack()
					log.Printf("Flush error %v\n", err)
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
		db.DLPrintf("NETCLNT", "Reader: about to ReadFrame from %v br:%p\n", nc.Dst(), br)
		frame, err := npcodec.ReadFrame(br)
		db.DLPrintf("NETCLNT", "Reader: ReadFrame from %v br:%p\n", nc.Dst(), br)
		// On connection error, retry
		if err == io.EOF || (err != nil && strings.Contains(err.Error(), "connection reset by peer")) {
			db.DLPrintf("NETCLNT", "Reader: NetClntection error to %v\n", nc.Dst())
			nc.resetConnection(br, bw)
			// Get the br for the latest connection
			br, bw, err = nc.getBufio()
			// If none was available, close the conn
			if err != nil {
				db.DLPrintf("NETCLNT", "Reader: no viable connections: %v", err)
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
				db.DLPrintf("NETCLNT", "Reader: from %v %v\n", nc.Dst(), fcall)
				nc.mu.Lock()
				if !nc.closed {
					rpc.replych <- &Reply{fcall, nil}
				}
				nc.mu.Unlock()
			}
		}
	}
}
