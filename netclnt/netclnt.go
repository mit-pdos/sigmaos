package netclnt

import (
	"bufio"
	"net"
	"sync"

	db "sigmaos/debug"
	"sigmaos/delay"
	"sigmaos/fcall"
	"sigmaos/sessconnclnt"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

//
// TCP connection which sends and receiveds RPCs to/from a single server.
//

type NetClnt struct {
	mu     sync.Mutex
	sconn  sessconnclnt.Conn
	conn   net.Conn
	addr   string
	closed bool
	br     *bufio.Reader
	bw     *bufio.Writer
}

func MakeNetClnt(sconn sessconnclnt.Conn, addrs []string) (*NetClnt, *fcall.Err) {
	db.DPrintf("NETCLNT", "mkNetClnt to %v\n", addrs)
	nc := &NetClnt{}
	nc.sconn = sconn
	err := nc.connect(addrs)
	if err != nil {
		db.DPrintf("NETCLNT_ERR", "MakeNetClnt connect %v err %v\n", addrs, err)
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
	nc.Close()
	nc.sconn.Reset()
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

func (nc *NetClnt) connect(addrs []string) *fcall.Err {
	for _, addr := range addrs {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			continue
		}
		nc.conn = c
		nc.addr = addr
		nc.br = bufio.NewReaderSize(c, sp.Conf.Conn.MSG_LEN)
		nc.bw = bufio.NewWriterSize(c, sp.Conf.Conn.MSG_LEN)
		db.DPrintf("NETCLNT", "NetClnt connected %v -> %v bw:%p, br:%p\n", c.LocalAddr(), nc.addr, nc.bw, nc.br)
		return nil
	}
	db.DPrintf("NETCLNT_ERR", "NetClnt unable to connect to any of %v\n", addrs)
	return fcall.MkErr(fcall.TErrUnreachable, "no connection")
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
		db.DPrintf("NETCLNT_ERR", "Error Send on closed channel to %v\n", nc.Dst())
		return
	}

	// Otherwise, marshall and write the fcall.
	err := spcodec.MarshalFcallMsg(rpc.Req, nc.bw)
	if err != nil {
		db.DPrintf("NETCLNT_ERR", "Send: NetClnt error to %v: %v", nc.Dst(), err)
		// The only error code we expect here is TErrUnreachable
		if err.Code() != fcall.TErrUnreachable {
			db.DFatalf("Unexpected error in netclnt.writer: %v", err)
		}
		return
	}
	error := nc.bw.Flush()
	if error != nil {
		db.DPrintf("NETCLNT_ERR", "Flush error cli %v to srv %v err %v\n", nc.Src(), nc.Dst(), error)
	}
}

func (nc *NetClnt) recv() (*sp.FcallMsg, *fcall.Err) {
	frame, err := spcodec.ReadFrame(nc.br)
	if err != nil {
		db.DPrintf("NETCLNT_ERR", "recv: ReadFrame cli %v from %v error %v\n", nc.Src(), nc.Dst(), err)
		nc.Close()
		return nil, fcall.MkErr(fcall.TErrUnreachable, nc.Src()+"->"+nc.Dst())
	}
	fm, err := spcodec.UnmarshalFcallMsg(frame)
	if err != nil {
		db.DFatalf("unmarshal fcall in NetClnt.recv %v", err)
	}
	db.DPrintf("NETCLNT", "recv %v from %v\n", fm, nc.Dst())
	return fm, nil
}

// Reader loop. The reader is in charge of resetting the connection if an error
// occurs.
func (nc *NetClnt) reader() {
	for true {
		// Receive the next reply.
		reply, err := nc.recv()
		if err != nil {
			db.DPrintf("NETCLNT_ERR", "error %v reader RPC to %v, trying reconnect", err, nc.addr)
			nc.reset()
			break
		}
		nc.sconn.CompleteRPC(reply, err)
		if nc.isClosed() {
			db.DPrintf("NETCLNT_ERR", "reader from %v to %v, closed", nc.Src(), nc.Dst())
			break
		}
	}
}
