package srv

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
	rpcchan "sigmaos/rpc/clnt/channel"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

type DelegatedRPCReplyTable struct {
	mu   sync.Mutex
	reps map[sp.Tpid]*RPCReplies
}

func NewDelegatedRPCReplyTable() *DelegatedRPCReplyTable {
	return &DelegatedRPCReplyTable{
		reps: make(map[sp.Tpid]*RPCReplies),
	}
}

type RPCReplies struct {
	mu       sync.Mutex
	channels map[string]rpcchan.RPCChannel
	cond     *sync.Cond
	done     []bool
	results  []sessp.IoVec
	errors   []error
}

func NewRPCReplies(rpcs []*proc.InitializationRPC) *RPCReplies {
	reps := &RPCReplies{
		channels: make(map[string]rpcchan.RPCChannel),
		done:     make([]bool, len(rpcs)),
		results:  make([]sessp.IoVec, len(rpcs)),
		errors:   make([]error, len(rpcs)),
	}
	reps.cond = sync.NewCond(&reps.mu)
	return reps
}

func (reps *RPCReplies) GetRPCChannel(pn string) (rpcchan.RPCChannel, bool) {
	reps.mu.Lock()
	defer reps.mu.Unlock()

	ch, ok := reps.channels[pn]
	return ch, ok
}

func (reps *RPCReplies) PutRPCChannel(pn string, ch rpcchan.RPCChannel) {
	reps.mu.Lock()
	defer reps.mu.Unlock()

	reps.channels[pn] = ch
}

// Insert the reply for a delegated RPC. Unblocks any waiters on the reply
func (reps *RPCReplies) InsertReply(idx uint64, iov sessp.IoVec, err error) {
	reps.mu.Lock()
	defer reps.mu.Unlock()

	i := int(idx)
	reps.done[i] = true
	reps.results[i] = iov
	reps.errors[i] = err
	reps.cond.Broadcast()
}

// Retrieve the reply for a delegated RPC. Blocks until the reply materializes
func (reps *RPCReplies) GetReply(idx uint64) (sessp.IoVec, error) {
	reps.mu.Lock()
	defer reps.mu.Unlock()

	for !reps.done[idx] {
		reps.cond.Wait()
	}
	i := int(idx)
	return reps.results[i], reps.errors[i]
}

func (tab *DelegatedRPCReplyTable) getReplies(pid sp.Tpid) *RPCReplies {
	tab.mu.Lock()
	defer tab.mu.Unlock()

	reps, ok := tab.reps[pid]
	// Sanity check
	if !ok {
		db.DFatalf("GetReplies unknown proc: %v", pid)
	}
	return reps
}

func (tab *DelegatedRPCReplyTable) InsertReply(pid sp.Tpid, rpcIdx uint64, iov sessp.IoVec, err error) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.InsertReply(%v)", pid, rpcIdx)
	reps := tab.getReplies(pid)
	reps.InsertReply(rpcIdx, iov, err)
}

func (tab *DelegatedRPCReplyTable) GetReply(pid sp.Tpid, rpcIdx uint64) (sessp.IoVec, error) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetReply(%v)", pid, rpcIdx)
	defer db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetReply(%v) done", pid, rpcIdx)

	reps := tab.getReplies(pid)
	return reps.GetReply(rpcIdx)
}

func (tab *DelegatedRPCReplyTable) GetRPCChannel(pid sp.Tpid, pn string) (rpcchan.RPCChannel, bool) {
	reps := tab.getReplies(pid)
	return reps.GetRPCChannel(pn)
}

func (tab *DelegatedRPCReplyTable) PutRPCChannel(pid sp.Tpid, pn string, ch rpcchan.RPCChannel) {
	reps := tab.getReplies(pid)
	reps.PutRPCChannel(pn, ch)
}

func (tab *DelegatedRPCReplyTable) NewProc(p *proc.Proc) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.NewProc", p.GetPid())

	tab.mu.Lock()
	defer tab.mu.Unlock()

	// Sanity check
	if _, ok := tab.reps[p.GetPid()]; ok {
		db.DFatalf("NewProc twice: %v", p.GetPid())
	}
	tab.reps[p.GetPid()] = NewRPCReplies(p.GetInitRPCs())
}

func (tab *DelegatedRPCReplyTable) DelProc(pid sp.Tpid) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.DelProc", pid)

	tab.mu.Lock()
	defer tab.mu.Unlock()

	// Sanity check
	if _, ok := tab.reps[pid]; !ok {
		db.DFatalf("Delete unknown proc: %v", pid)
	}
	delete(tab.reps, pid)
}
