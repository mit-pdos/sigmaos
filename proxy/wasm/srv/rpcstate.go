package srv

import (
	"sync"
	"time"

	db "sigmaos/debug"
	rpcchan "sigmaos/rpc/clnt/channel"
	sprpcchan "sigmaos/rpc/clnt/channel/spchannel"
	sessp "sigmaos/session/proto"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/procclnt"
	"sigmaos/util/perf"
)

type RPCState struct {
	mu                        sync.Mutex
	cond                      *sync.Cond
	channels                  map[string]rpcchan.RPCChannel
	channelCreationInProgress map[string]bool
	channelCreationErrors     map[string]error
	done                      map[uint64]bool
	results                   map[uint64]*sessp.IoVec
	errors                    map[uint64]error
}

func NewRPCState() *RPCState {
	rpcst := &RPCState{
		channels:                  make(map[string]rpcchan.RPCChannel),
		channelCreationInProgress: make(map[string]bool),
		channelCreationErrors:     make(map[string]error),
		done:                      make(map[uint64]bool),
		results:                   make(map[uint64]*sessp.IoVec),
		errors:                    make(map[uint64]error),
	}
	rpcst.cond = sync.NewCond(&rpcst.mu)
	return rpcst
}

func (rpcs *RPCState) GetRPCChannel(sc *sigmaclnt.SigmaClnt, rpcIdx uint64, pn string) (rpcchan.RPCChannel, error) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	err, ok := rpcs.channelCreationErrors[pn]
	if ok && err != nil {
		db.DPrintf(db.WASMD_ERR, "[%v] delRPC(%v) previous channel creation failed pn:%v err:%v", sc.ProcEnv().GetPID(), rpcIdx, pn, err)
		return nil, err
	}

	ch, ok := rpcs.channels[pn]
	if ok {
		db.DPrintf(db.WASMD, "[%v] delRPC(%v) reuse cached channel for: %v", sc.ProcEnv().GetPID(), rpcIdx, pn)
		return ch, nil
	}

	if ok := rpcs.channelCreationInProgress[pn]; !ok {
		start := time.Now()
		db.DPrintf(db.WASMD, "[%v] delRPC(%v) create new channel pn:%v", sc.ProcEnv().GetPID(), rpcIdx, pn)
		rpcs.channelCreationInProgress[pn] = true
		rpcs.mu.Unlock()

		// A ~local pn (e.g. from a cosandbox manifest, which is built before
		// the proc is placed) resolves against the proc's kernel ID, since
		// that is the key its parent cached under.
		if ep, ok := procclnt.LookupCachedEndpoint(sc.ProcEnv(), pn); ok {
			db.DPrintf(db.WASMD, "[%v] delRPC(%v) create channel EP cached pn:%v", sc.ProcEnv().GetPID(), rpcIdx, pn)
			ch, err = sprpcchan.NewSPChannelEndpoint(sc.FsLib, pn, ep, false)
			if err != nil {
				db.DPrintf(db.WASMD_ERR, "Err create mounted RPC channel (%v -> %v): %v", pn, ep, err)
			}
		} else {
			db.DPrintf(db.WASMD, "[%v] delRPC(%v) create channel no EP pn:%v", sc.ProcEnv().GetPID(), rpcIdx, pn)
			ch, err = sprpcchan.NewSPChannel(sc.FsLib, pn, false)
			if err != nil {
				db.DPrintf(db.WASMD_ERR, "Err create unmounted RPC channel (%v): %v", pn, err)
			}
		}

		rpcs.mu.Lock()
		rpcs.channels[pn] = ch
		rpcs.channelCreationErrors[pn] = err
		delete(rpcs.channelCreationInProgress, pn)
		rpcs.cond.Broadcast()
		perf.LogSpawnLatency("WASMd.ConnectionSetup", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
	} else {
		db.DPrintf(db.WASMD, "[%v] delRPC(%v) wait for channel creation pn:%v", sc.ProcEnv().GetPID(), rpcIdx, pn)
		for rpcs.channelCreationInProgress[pn] {
			rpcs.cond.Wait()
		}
	}
	return rpcs.channels[pn], rpcs.channelCreationErrors[pn]
}

func (rpcs *RPCState) InsertReply(idx uint64, iov *sessp.IoVec, err error) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	if rpcs.done[idx] {
		db.DFatalf("Err double-insert RPC(%v) reply", idx)
	}
	rpcs.done[idx] = true
	rpcs.results[idx] = iov
	rpcs.errors[idx] = err
	rpcs.cond.Broadcast()
}

func (rpcs *RPCState) GetReply(idx uint64) (*sessp.IoVec, error) {
	rpcs.mu.Lock()
	defer rpcs.mu.Unlock()

	for !rpcs.done[idx] {
		rpcs.cond.Wait()
	}
	return rpcs.results[idx], rpcs.errors[idx]
}
