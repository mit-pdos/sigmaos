package netclnt

import (
	"time"

	"sigmaos/serr"
	"sigmaos/sessconn"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

type Reply struct {
	f   []byte
	iov sessp.IoVec
	err *serr.Err
}

type Rpc struct {
	start  time.Time
	addrs  sp.Taddrs
	Req    *sessconn.PartMarshaledMsg
	ReplyC chan *Reply
}

func NewRpc(addrs sp.Taddrs, req *sessconn.PartMarshaledMsg, start time.Time) *Rpc {
	rpc := &Rpc{
		start:  start,
		addrs:  addrs,
		Req:    req,
		ReplyC: make(chan *Reply),
	}
	return rpc
}

// Wait for a reply
func (rpc *Rpc) Await() (*sessp.FcallMsg, *serr.Err) {
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, serr.NewErr(serr.TErrUnreachable, rpc.addrs)
	}

	// Unmarshal reply, now on the receiver thread.
	fm := spcodec.UnmarshalFcall(reply.f, reply.iov)
	return fm, reply.err
}

// Complete a reply
func (rpc *Rpc) Complete(f []byte, iov sessp.IoVec, err *serr.Err) {
	rpc.ReplyC <- &Reply{f, iov, err}
	close(rpc.ReplyC)
}

// Abort an RPC, because the connection has closed.
func (rpc *Rpc) Abort() {
	close(rpc.ReplyC)
}

// Total delay since the sessclnt layer first saw this RPC.
func (rpc *Rpc) TotalDelay() time.Duration {
	return time.Since(rpc.start)
}
