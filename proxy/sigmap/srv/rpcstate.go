package srv

import (
	"sync"

	"sigmaos/proc"
	rpcchan "sigmaos/rpc/clnt/channel"
	sessp "sigmaos/session/proto"
)

type RPCState struct {
	mu       sync.Mutex
	channels map[string]rpcchan.RPCChannel
	cond     *sync.Cond
	done     []bool
	results  []sessp.IoVec
	errors   []error
}

func NewRPCState(rpcs []*proc.InitializationRPC) *RPCState {
	rpcst := &RPCState{
		channels: make(map[string]rpcchan.RPCChannel),
		done:     make([]bool, len(rpcs)),
		results:  make([]sessp.IoVec, len(rpcs)),
		errors:   make([]error, len(rpcs)),
	}
	rpcst.cond = sync.NewCond(&rpcst.mu)
	return rpcst
}

func (rpcs *RPCState) GetRPCChannel(pn string) (rpcchan.RPCChannel, bool) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	ch, ok := rpcs.channels[pn]
	return ch, ok
}

func (rpcs *RPCState) PutRPCChannel(pn string, ch rpcchan.RPCChannel) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	rpcs.channels[pn] = ch
}

// Insert the reply for a delegated RPC. Unblocks any waiters on the reply
func (rpcs *RPCState) InsertReply(idx uint64, iov sessp.IoVec, err error) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	i := int(idx)
	rpcs.done[i] = true
	rpcs.results[i] = iov
	rpcs.errors[i] = err
	rpcs.cond.Broadcast()
}

// Retrieve the reply for a delegated RPC. Blocks until the reply materializes
func (rpcs *RPCState) GetReply(idx uint64) (sessp.IoVec, error) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	for !rpcs.done[idx] {
		rpcs.cond.Wait()
	}
	i := int(idx)
	return rpcs.results[i], rpcs.errors[i]
}
