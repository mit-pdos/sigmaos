package srv

import (
	"sync"

	db "sigmaos/debug"
	rpcchan "sigmaos/rpc/clnt/channel"
	sprpcchan "sigmaos/rpc/clnt/channel/spchannel"
	sessp "sigmaos/session/proto"
	"sigmaos/sigmaclnt"
)

type RPCState struct {
	mu                        sync.Mutex
	cond                      *sync.Cond
	channels                  map[string]rpcchan.RPCChannel
	channelCreationInProgress map[string]bool
	channelCreationErrors     map[string]error
	done                      map[uint64]bool
	results                   map[uint64]sessp.IoVec
	errors                    map[uint64]error
}

func NewRPCState() *RPCState {
	rpcst := &RPCState{
		channels:                  make(map[string]rpcchan.RPCChannel),
		channelCreationInProgress: make(map[string]bool),
		channelCreationErrors:     make(map[string]error),
		done:                      make(map[uint64]bool),
		results:                   make(map[uint64]sessp.IoVec),
		errors:                    make(map[uint64]error),
	}
	rpcst.cond = sync.NewCond(&rpcst.mu)
	return rpcst
}

func (rpcs *RPCState) GetRPCChannel(sc *sigmaclnt.SigmaClnt, rpcIdx uint64, pn string) (rpcchan.RPCChannel, error) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	// Check if a prior attempt to create the channel resulted in an error
	err, ok := rpcs.channelCreationErrors[pn]
	if ok && err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] delRPC(%v) previous channel creation failed pn:%v err:%v", sc.ProcEnv().GetPID(), rpcIdx, pn, err)
		return nil, err
	}

	// Check if the channel was successfully created
	ch, ok := rpcs.channels[pn]
	if ok {
		// Channel already exists
		db.DPrintf(db.SPPROXYSRV, "[%v] delRPC(%v) reuse cached channel for: %v", sc.ProcEnv().GetPID(), rpcIdx, pn)
		return ch, nil
	} else {
		// If this is the first attempt to get this channel, create it
		if ok := rpcs.channelCreationInProgress[pn]; !ok {
			db.DPrintf(db.SPPROXYSRV, "[%v] delRPC(%v) create new channel pn:%v", sc.ProcEnv().GetPID(), rpcIdx, pn)
			// Note that channel creation is in-progress
			rpcs.channelCreationInProgress[pn] = true

			// Release the lock so that other parallel RPCs can make progress
			rpcs.mu.Unlock()

			// If the proc env has an endpoint cached, use it to make the channel
			if ep, ok := sc.ProcEnv().GetCachedEndpoint(pn); ok {
				db.DPrintf(db.SPPROXYSRV, "[%v] delRPC(%v) create channel EP cached pn:%v", sc.ProcEnv().GetPID(), rpcIdx, pn)
				ch, err = sprpcchan.NewSPChannelEndpoint(sc.FsLib, pn, ep)
				if err != nil {
					db.DPrintf(db.SPPROXYSRV_ERR, "Err create mounted RPC channel to run delRPCs (%v -> %v): %v", pn, ep, err)
				}
			} else {
				db.DPrintf(db.SPPROXYSRV, "[%v] delRPC(%v) create channel no EP pn:%v", sc.ProcEnv().GetPID(), rpcIdx, pn)
				ch, err = sprpcchan.NewSPChannel(sc.FsLib, pn)
				if err != nil {
					db.DPrintf(db.SPPROXYSRV_ERR, "Err create unmounted RPC channel to run delRPCs (%v): %v", pn, err)
				}
			}

			// Lock the mutex again to register channel creation completion
			rpcs.mu.Lock()

			// Store the channel for later reuse
			rpcs.channels[pn] = ch
			// Note any errors during channel creation
			rpcs.channelCreationErrors[pn] = err
			// Clean up in-progress creation marker
			delete(rpcs.channelCreationInProgress, pn)
			// Signal waiters that the channel (or error) is ready
			rpcs.cond.Broadcast()
		} else {
			db.DPrintf(db.SPPROXYSRV, "[%v] delRPC(%v) wait for channel creation pn:%v", sc.ProcEnv().GetPID(), rpcIdx, pn)
			// Wait until channel creation finished
			for rpcs.channelCreationInProgress[pn] {
				rpcs.cond.Wait()
			}
			db.DPrintf(db.SPPROXYSRV, "[%v] delRPC(%v) done waiting for channel creation pn:%v err %v", sc.ProcEnv().GetPID(), rpcIdx, pn, err)
		}
	}
	return rpcs.channels[pn], rpcs.channelCreationErrors[pn]
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
