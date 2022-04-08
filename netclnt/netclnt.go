package netclnt

import (
	"bufio"
	"net"
	"sync"

	db "ulambda/debug"
	"ulambda/delay"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/sessconnclnt"
)

//
// TCP connection which sends and receiveds RPCs to/from a single server.
//

// XXX duplicate
const (
	Msglen = 64 * 1024
)

type NetClnt struct {
	mu     sync.Mutex
	sconn  sessconnclnt.Conn
	conn   net.Conn
	addr   string
	closed bool
	br     *bufio.Reader
	bw     *bufio.Writer
}

func MakeNetClnt(sconn sessconnclnt.Conn, addr string) (*NetClnt, *np.Err) {
	db.DPrintf("NETCLNT", "mkNetClnt to %v\n", addr)
	nc := &NetClnt{}
	nc.sconn = sconn
	nc.addr = addr
	err := nc.connect()
	if err != nil {
		db.DPrintf("NETCLNT_ERR", "MakeNetClnt connect %v err %v\n", addr, err)
		return nil, err
	}
	go nc.reader()
	return nc, nil
}

func (nc *NetClnt) Dst() string {
	return nc.conn.RemoteAddr().String()
}

func (nc *NetClnt) Src() string {
	return nc.conn.LocalAddr().String()
}

// Reset the connection and upcall into the layer above to let it know the
// connection was reset.
func (nc *NetClnt) reset() {
	nc.sconn.Reset()
	nc.Close()
}

func (nc *NetClnt) Close() {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if !nc.closed {
		db.DPrintf("NETCLNT", "Close conn to %v\n", nc.Dst())
		nc.conn.Close()
	}
	nc.closed = true
}

func (nc *NetClnt) isClosed() bool {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.closed
}

func (nc *NetClnt) connect() *np.Err {
	c, err := net.Dial("tcp", nc.addr)
	if err != nil {
		db.DPrintf("NETCLNT_ERR", "NetClnt connect to %v err %v\n", nc.addr, err)
		return np.MkErr(np.TErrUnreachable, "no connection")
	}
	nc.conn = c
	nc.br = bufio.NewReaderSize(c, Msglen)
	nc.bw = bufio.NewWriterSize(c, Msglen)
	db.DPrintf("NETCLNT", "NetClnt connected %v -> %v bw:%p, br:%p\n", c.LocalAddr(), nc.addr, nc.bw, nc.br)
	return nil
}

// Try to send a request to the server. If an error occurs, close the
// underlying TCP connection so that the reader is made aware of the error, and
// can upcall into the layer above as appropriate.
func (nc *NetClnt) Send(rpc *Rpc) {
	// maybe delay sending this RPC
	delay.MaybeDelayRPC()

	db.DPrintf("NETCLNT", "Send %v to %v\n", rpc.Req, nc.Dst())

	// If the connection has already been closed, return immediately.
	if nc.isClosed() {
		db.DPrintf("NETCLNT_ERR", "Error Send on closed channelcto %v\n", nc.Dst())
		return
	}

	// Otherwise, marshall and write the fcall.
	err := npcodec.MarshalFcall(rpc.Req, nc.bw)
	if err != nil {
		db.DPrintf("NETCLNT_ERR", "Send: NetClnt error to %v: %v", nc.Dst(), err)
		nc.Close()
		// The only error code we expect here is TErrUnreachable
		if err.Code() != np.TErrUnreachable {
			db.DFatalf("Unexpected error in netclnt.writer: %v", err)
		}
		return
	}
	error := nc.bw.Flush()
	if error != nil {
		db.DPrintf("NETCLNT_ERR", "Flush error cli %v to srv %v err %v\n", nc.Src(), nc.Dst(), error)
		nc.Close()
	}
}

func (nc *NetClnt) recv() (*np.Fcall, *np.Err) {
	frame, err := npcodec.ReadFrame(nc.br)
	if err != nil {
		db.DPrintf("NETCLNT_ERR", "recv: ReadFrame cli %v from %v error %v\n", nc.Src(), nc.Dst(), err)
		nc.Close()
		return nil, np.MkErr(np.TErrUnreachable, nc.Src()+"->"+nc.Dst())
	}
	fcall, err := npcodec.UnmarshalFcall(frame)
	if err != nil {
		db.DFatalf("unmarshal fcall in NetClnt.recv %v", err)
	}
	db.DPrintf("NETCLNT", "recv %v from %v\n", fcall, nc.Dst())
	return fcall, nil
}

// Reader loop. The reader is in charge of resetting the connection if an error
// occurs.
func (nc *NetClnt) reader() {
	for !nc.isClosed() {
		// Receive the next reply.
		reply, err := nc.recv()
		if err != nil {
			db.DPrintf("NETCLNT", "error %v reader RPC to %v, trying reconnect", err, nc.addr)
			nc.reset()
			continue
		}
		nc.sconn.CompleteRPC(reply, err)
	}
}
