package srv

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
)

// Support for queueing procs before running them, because they share the same
// resource allocation
type ProcQueue struct {
	mu         sync.Mutex
	cond       *sync.Cond
	poolQueues map[uint64][]*proc.Proc
}

func newProcQueue() *ProcQueue {
	pq := &ProcQueue{
		poolQueues: make(map[uint64][]*proc.Proc),
	}
	pq.cond = sync.NewCond(&pq.mu)
	return pq
}

func (pq *ProcQueue) QueueProc(p *proc.Proc) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	qid := p.GetQueueableResourcePoolID()
	db.DPrintf(db.PROCD, "[%v] Queue proc qlen[%v] %v", p.GetPid(), qid, len(pq.poolQueues[qid]))
	// Append the proc to the queue
	pq.poolQueues[qid] = append(pq.poolQueues[qid], p)
	// Wait until this is the only proc on the queue. When this is the case, the
	// proc can run
	for len(pq.poolQueues[qid]) > 1 {
		pq.cond.Wait()
	}
	db.DPrintf(db.PROCD, "[%v] Done queueing proc qlen[%v]", p.GetPid(), qid, len(pq.poolQueues[qid]))
}

func (pq *ProcQueue) ProcDone(p *proc.Proc) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	qid := p.GetQueueableResourcePoolID()
	db.DPrintf(db.PROCD, "[%v] Proc done qlen[%v] %v", p.GetPid(), qid, len(pq.poolQueues[qid]))
	// Dequeue the proc
	var oldP *proc.Proc
	oldP, pq.poolQueues[qid] = pq.poolQueues[qid][0], pq.poolQueues[qid][1:]
	// Sanity check
	if oldP != p {
		db.DFatalf("ProcDone and running proc don't match [%v]: %v != %v", qid, p.GetPid(), oldP.GetPid())
	}
	// If the queue is now empty, clean it up
	if len(pq.poolQueues[qid]) == 0 {
		db.DPrintf(db.PROCD, "Queue %v empty, deleting", qid)
		delete(pq.poolQueues, qid)
	}
	// Wait up waiters
	pq.cond.Signal()
}
