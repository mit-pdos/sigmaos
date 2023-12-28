package demux

import (
	"fmt"
	"sync"

	"sigmaos/serr"
	"sigmaos/sessp"
)

// One rpc struct per outstanding RPC
type rpc struct {
	ch      chan error
	seqno   sessp.Tseqno
	request []byte
	reply   []byte
}

func (r *rpc) String() string {
	return fmt.Sprintf("{rpc %d %d %d}", r.seqno, len(r.request), len(r.reply))
}

// Map of outstanding RPCs indexed by sequence number
type rpcMap struct {
	sync.Mutex
	closed bool
	rpcs   map[sessp.Tseqno]*rpc
}

func newRpcMap() *rpcMap {
	return &rpcMap{rpcs: make(map[sessp.Tseqno]*rpc)}
}

func (rm *rpcMap) close() {
	rm.Lock()
	defer rm.Unlock()

	rm.closed = true
}

func (rm *rpcMap) outstanding() []sessp.Tseqno {
	rm.Lock()
	defer rm.Unlock()

	o := make([]sessp.Tseqno, 0, len(rm.rpcs))
	for k, _ := range rm.rpcs {
		o = append(o, k)
	}
	return o
}

func (rm *rpcMap) put(seqno sessp.Tseqno, rpc *rpc) error {
	rm.Lock()
	defer rm.Unlock()
	if rm.closed {
		return serr.NewErr(serr.TErrUnreachable, "dmxclnt")
	}
	rm.rpcs[seqno] = rpc
	return nil
}

func (rm *rpcMap) remove(seqno sessp.Tseqno) (*rpc, bool) {
	rm.Lock()
	defer rm.Unlock()

	last := false
	if rpc, ok := rm.rpcs[seqno]; ok {
		delete(rm.rpcs, seqno)
		if len(rm.rpcs) == 0 && rm.closed {
			last = true
		}
		return rpc, last
	}
	return nil, last
}
