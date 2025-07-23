package srv

import (
	"sync"

	rpcchan "sigmaos/rpc/clnt/channel"
	sessp "sigmaos/session/proto"
)

type RPCState struct {
	mu       sync.Mutex
	channels map[string]rpcchan.RPCChannel
	cond     *sync.Cond
	done     map[uint64]bool
	results  map[uint64]sessp.IoVec
	errors   map[uint64]error
}

func NewRPCState() *RPCState {
	rpcst := &RPCState{
		channels: make(map[string]rpcchan.RPCChannel),
		done:     make(map[uint64]bool),
		results:  make(map[uint64]sessp.IoVec),
		errors:   make(map[uint64]error),
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

	rpcs.done[idx] = true
	rpcs.results[idx] = iov
	rpcs.errors[idx] = err
	rpcs.cond.Broadcast()
}

// Retrieve the reply for a delegated RPC. Blocks until the reply materializes
func (rpcs *RPCState) GetReply(idx uint64) (sessp.IoVec, error) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	for !rpcs.done[idx] {
		rpcs.cond.Wait()
	}
	return rpcs.results[idx], rpcs.errors[idx]
}
