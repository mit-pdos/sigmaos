package netclnt

import (
	"sigmaos/serr"
	"sigmaos/sessconn"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Reply struct {
	fm  *sessp.FcallMsg
	err *serr.Err
}

type Rpc struct {
	addrs  sp.Taddrs
	Req    *sessconn.PartMarshaledMsg
	ReplyC chan *Reply
}

func MakeRpc(addrs sp.Taddrs, fc *sessp.FcallMsg) *Rpc {
	rpc := &Rpc{}
	rpc.addrs = addrs
	rpc.Req = sessconn.MakePartMarshaledMsg(fc)
	rpc.ReplyC = make(chan *Reply)
	return rpc
}

// Wait for a reply
func (rpc *Rpc) Await() (*sessp.FcallMsg, *serr.Err) {
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, serr.MkErr(serr.TErrUnreachable, rpc.addrs)
	}
	return reply.fm, reply.err
}

// Complete a reply
func (rpc *Rpc) Complete(reply *sessp.FcallMsg, err *serr.Err) {
	rpc.ReplyC <- &Reply{reply, err}
	close(rpc.ReplyC)
}

// Abort an RPC, because the connection has closed.
func (rpc *Rpc) Abort() {
	close(rpc.ReplyC)
}
