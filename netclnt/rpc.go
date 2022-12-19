package netclnt

import (
	"sigmaos/fcall"
	sp "sigmaos/sigmap"
)

type Reply struct {
	fm  *sp.FcallMsg
	err *fcall.Err
}

type Rpc struct {
	addrs  []string
	Req    *sp.FcallMsg
	ReplyC chan *Reply
}

func MakeRpc(addrs []string, fc *sp.FcallMsg) *Rpc {
	rpc := &Rpc{}
	rpc.addrs = addrs
	rpc.Req = fc
	rpc.ReplyC = make(chan *Reply)
	return rpc
}

// Wait for a reply
func (rpc *Rpc) Await() (*sp.FcallMsg, *fcall.Err) {
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, fcall.MkErr(fcall.TErrUnreachable, rpc.addrs)
	}
	return reply.fm, reply.err
}

// Complete a reply
func (rpc *Rpc) Complete(reply *sp.FcallMsg, err *fcall.Err) {
	rpc.ReplyC <- &Reply{reply, err}
	close(rpc.ReplyC)
}

// Abort an RPC, because the connection has closed.
func (rpc *Rpc) Abort() {
	close(rpc.ReplyC)
}
