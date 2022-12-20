package netclnt

import (
	"sigmaos/sessp"
)

type Reply struct {
	fm  *sessp.FcallMsg
	err *sessp.Err
}

type Rpc struct {
	addrs  []string
	Req    *sessp.FcallMsg
	ReplyC chan *Reply
}

func MakeRpc(addrs []string, fc *sessp.FcallMsg) *Rpc {
	rpc := &Rpc{}
	rpc.addrs = addrs
	rpc.Req = fc
	rpc.ReplyC = make(chan *Reply)
	return rpc
}

// Wait for a reply
func (rpc *Rpc) Await() (*sessp.FcallMsg, *sessp.Err) {
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, sessp.MkErr(sessp.TErrUnreachable, rpc.addrs)
	}
	return reply.fm, reply.err
}

// Complete a reply
func (rpc *Rpc) Complete(reply *sessp.FcallMsg, err *sessp.Err) {
	rpc.ReplyC <- &Reply{reply, err}
	close(rpc.ReplyC)
}

// Abort an RPC, because the connection has closed.
func (rpc *Rpc) Abort() {
	close(rpc.ReplyC)
}
