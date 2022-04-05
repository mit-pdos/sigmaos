package netclnt

import (
	np "ulambda/ninep"
)

type Reply struct {
	fc  *np.Fcall
	err *np.Err
}

type Rpc struct {
	addrs  []string
	Req    *np.Fcall
	ReplyC chan *Reply
}

func MakeRpc(addrs []string, fc *np.Fcall) *Rpc {
	rpc := &Rpc{}
	rpc.addrs = addrs
	rpc.Req = fc
	rpc.ReplyC = make(chan *Reply)
	return rpc
}

// Wait for a reply
func (rpc *Rpc) Wait() (np.Tmsg, *np.Err) {
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, np.MkErr(np.TErrUnreachable, rpc.addrs)
	}
	return reply.fc.Msg, reply.err
}

// Complete a reply
func (rpc *Rpc) Complete(reply *np.Fcall, err *np.Err) {
	rpc.ReplyC <- &Reply{reply, err}
	close(rpc.ReplyC)
}

// Abort an RPC, because the connection has closed.
func (rpc *Rpc) Abort() {
	close(rpc.ReplyC)
}
