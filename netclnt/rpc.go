package netclnt

import (
	"sigmaos/fcall"
)

type Reply struct {
	fm  *fcall.FcallMsg
	err *fcall.Err
}

type Rpc struct {
	addrs  []string
	Req    *fcall.FcallMsg
	ReplyC chan *Reply
}

func MakeRpc(addrs []string, fc *fcall.FcallMsg) *Rpc {
	rpc := &Rpc{}
	rpc.addrs = addrs
	rpc.Req = fc
	rpc.ReplyC = make(chan *Reply)
	return rpc
}

// Wait for a reply
func (rpc *Rpc) Await() (*fcall.FcallMsg, *fcall.Err) {
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, fcall.MkErr(fcall.TErrUnreachable, rpc.addrs)
	}
	return reply.fm, reply.err
}

// Complete a reply
func (rpc *Rpc) Complete(reply *fcall.FcallMsg, err *fcall.Err) {
	rpc.ReplyC <- &Reply{reply, err}
	close(rpc.ReplyC)
}

// Abort an RPC, because the connection has closed.
func (rpc *Rpc) Abort() {
	close(rpc.ReplyC)
}
