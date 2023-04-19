package netclnt

import (
	"sigmaos/serr"
	"sigmaos/sessconn"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

type Reply struct {
	f   []byte
	d   []byte
	err *serr.Err
}

type Rpc struct {
	addrs  sp.Taddrs
	Req    *sessconn.PartMarshaledMsg
	ReplyC chan *Reply
}

func MakeRpc(addrs sp.Taddrs, req *sessconn.PartMarshaledMsg) *Rpc {
	rpc := &Rpc{}
	rpc.addrs = addrs
	rpc.Req = req
	rpc.ReplyC = make(chan *Reply)
	return rpc
}

// Wait for a reply
func (rpc *Rpc) Await() (*sessp.FcallMsg, *serr.Err) {
	reply, ok := <-rpc.ReplyC
	if !ok {
		return nil, serr.MkErr(serr.TErrUnreachable, rpc.addrs)
	}

	// Unmarshal reply, now on the receiver thread.
	fm := spcodec.UnmarshalFcallAndData(reply.f, reply.d)
	return fm, reply.err
}

// Complete a reply
func (rpc *Rpc) Complete(f []byte, d []byte, err *serr.Err) {
	rpc.ReplyC <- &Reply{f, d, err}
	close(rpc.ReplyC)
}

// Abort an RPC, because the connection has closed.
func (rpc *Rpc) Abort() {
	close(rpc.ReplyC)
}
