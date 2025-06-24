package srv

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
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
	mu      sync.Mutex
	cond    *sync.Cond
	done    []bool
	results []sessp.IoVec
}

func NewRPCReplies(rpcs []*proc.InitializationRPC) *RPCReplies {
	reps := &RPCReplies{
		done:    make([]bool, len(rpcs)),
		results: make([]sessp.IoVec, len(rpcs)),
	}
	reps.cond = sync.NewCond(&reps.mu)
	return reps
}

// Insert the reply for a delegated RPC. Unblocks any waiters on the reply
func (reps *RPCReplies) InsertReply(idx uint64, iov sessp.IoVec) {
	reps.mu.Lock()
	defer reps.mu.Unlock()

	i := int(idx)
	reps.done[i] = true
	reps.results[i] = iov
	reps.cond.Broadcast()
}

// Retrieve the reply for a delegated RPC. Blocks until the reply materializes
func (reps *RPCReplies) GetReply(idx uint64) sessp.IoVec {
	reps.mu.Lock()
	defer reps.mu.Unlock()

	for !reps.done[idx] {
		reps.cond.Wait()
	}
	i := int(idx)
	return reps.results[i]
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

func (tab *DelegatedRPCReplyTable) InsertReply(pid sp.Tpid, rpcIdx uint64, iov sessp.IoVec) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.InsertReply(%v)", pid, rpcIdx)
	reps := tab.getReplies(pid)
	reps.InsertReply(rpcIdx, iov)
}

func (tab *DelegatedRPCReplyTable) GetReply(pid sp.Tpid, rpcIdx uint64) sessp.IoVec {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetReply(%v)", pid, rpcIdx)
	defer db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetReply(%v) done", pid, rpcIdx)

	reps := tab.getReplies(pid)
	return reps.GetReply(rpcIdx)
}

func (tab *DelegatedRPCReplyTable) NewProc(pe *proc.ProcEnv) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.NewProc", pe.GetPID())

	tab.mu.Lock()
	defer tab.mu.Unlock()

	// Sanity check
	if _, ok := tab.reps[pe.GetPID()]; ok {
		db.DFatalf("NewProc twice: %v", pe.GetPID())
	}
	tab.reps[pe.GetPID()] = NewRPCReplies(pe.GetInitRPCs())
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
