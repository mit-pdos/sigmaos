package netclnt

import (
	"bufio"
	"log"
	"net"
	"runtime/debug"
	"sync"

	db "ulambda/debug"
	"ulambda/delay"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

//
// Multiplexes RPCs onto a single network connection to server
//

// XXX duplicate
const (
	Msglen = 64 * 1024
)

type Reply struct {
	Fc  *np.Fcall
	Err *np.Err
}

type Rpc struct {
	Req    *np.Fcall
	Done   bool
	ReplyC chan *Reply
}

func MakeRpc(fc *np.Fcall) *Rpc {
	rpc := &Rpc{}
	rpc.Req = fc
	rpc.ReplyC = make(chan *Reply)
	return rpc
}

type NetClnt struct {
	mu     sync.Mutex
	conn   net.Conn
	addr   string
	closed bool
	br     *bufio.Reader
	bw     *bufio.Writer
}

func MakeNetClnt(addr string) (*NetClnt, *np.Err) {
	db.DLPrintf("NETCLNT", "mkNetClnt to %v\n", addr)
	nc := &NetClnt{}
	nc.addr = addr
	err := nc.connect()
	if err != nil {
		db.DLPrintf("NETCLNT_ERR", "MakeNetClnt connect %v err %v\n", addr, err)
		return nil, err
	}

	return nc, nil
}

func (nc *NetClnt) Dst() string {
	return nc.conn.RemoteAddr().String()
}

func (nc *NetClnt) Src() string {
	return nc.conn.LocalAddr().String()
}

func (nc *NetClnt) Close() {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	db.DLPrintf("NETCLNT", "Close conn to %v\n", nc.Dst())
	wasClosed := nc.closed
	nc.closed = true

	// Close the requests channel asynchronously so we don't deadlock
	if !wasClosed {
		nc.conn.Close()
	}
}

func (nc *NetClnt) connect() *np.Err {
	c, err := net.Dial("tcp", nc.addr)
	if err != nil {
		db.DLPrintf("NETCLNT_ERR", "NetClnt to %v err %v\n", nc.addr, err)
		return np.MkErr(np.TErrUnreachable, "no connection")
	}
	nc.conn = c
	nc.br = bufio.NewReaderSize(c, Msglen)
	nc.bw = bufio.NewWriterSize(c, Msglen)
	db.DLPrintf("NETCLNT", "NetClnt %v -> %v bw:%p, br:%p\n", c.LocalAddr(), nc.addr, nc.bw, nc.br)
	return nil
}

func (nc *NetClnt) Send(rpc *Rpc) *np.Err {
	db.DLPrintf("RPC", "req %v to %v\n", rpc, nc.Dst())
	// Tag is unused for now.
	rpc.Req.Tag = 0

	// maybe delay sending this RPC
	delay.MaybeDelayRPC()

	nc.mu.Lock()
	closed := nc.closed
	nc.mu.Unlock()
	if closed {
		db.DLPrintf("NETCLNT_ERR", "Error ch to %v closed\n", nc.Dst())
		return np.MkErr(np.TErrUnreachable, nc.Dst())
	}
	return nc.write(rpc)
}

func (nc *NetClnt) Recv() (*np.Fcall, *np.Err) {
	return nc.read()
}

func (nc *NetClnt) write(rpc *Rpc) *np.Err {
	err := npcodec.MarshalFcall(rpc.Req, nc.bw)
	if err != nil {
		db.DLPrintf("NETCLNT_ERR", "Writer: NetClnt error to %v: %v", nc.Dst(), err)
		// XXX Connection reset by peer?
		if err.Code() == np.TErrUnreachable {
			db.DLPrintf("NETCLNT_ERR", "Writer: NetClntection error cli %v to %v err %v\n", nc.Src(), nc.Dst(), err)
			nc.Close()
			return np.MkErr(np.TErrUnreachable, nc.Dst())
		} else {
			log.Fatalf("FATAL error in netclnt.writer: %v", err)
		}
	} else {
		error := nc.bw.Flush()
		if error != nil {
			stacktrace := debug.Stack()
			db.DLPrintf("NETCLNT_ERR", "%v\nFlush error cli %v to srv %v err %v\n", string(stacktrace), nc.Src(), nc.Dst(), error)
			nc.Close()
			return np.MkErr(np.TErrUnreachable, nc.Dst())
		}
	}
	return nil
}

func (nc *NetClnt) read() (*np.Fcall, *np.Err) {
	frame, err := npcodec.ReadFrame(nc.br)
	if err != nil {
		db.DLPrintf("NETCLNT_ERR", "Reader: ReadFrame cli %v from %v error %v\n", nc.Src(), nc.Dst(), err)
		nc.Close()
		return nil, np.MkErr(np.TErrUnreachable, nc.Src()+"->"+nc.Dst())
	}
	fcall, err := npcodec.UnmarshalFcall(frame)
	if err != nil {
		log.Fatalf("FATAL: unmarshal fcall in NetClnt.reader %v", err)
		db.DLPrintf("NETCLNT_ERR", "Reader: Unmarshal error %v\n", err)
	}
	return fcall, nil
}
