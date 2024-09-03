package procqsrv

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"sigmaos/chunk"
	"sigmaos/chunkclnt"
	"sigmaos/chunksrv"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procfs"
	proto "sigmaos/procqsrv/proto"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

const (
	GET_PROC_TIMEOUT = 50 * time.Millisecond
)

type ProcQ struct {
	mu        sync.Mutex
	realmMu   sync.RWMutex
	cond      *sync.Cond
	sc        *sigmaclnt.SigmaClnt
	qs        map[sp.Trealm]*Queue
	realms    []sp.Trealm
	rr        *RealmRR
	qlen      int // Aggregate queue length, across all queues
	tot       atomic.Int64
	realmbins *chunkclnt.RealmBinPaths
}

type QDir struct {
	pq *ProcQ
}

func NewProcQ(sc *sigmaclnt.SigmaClnt) *ProcQ {
	pq := &ProcQ{
		sc:        sc,
		qs:        make(map[sp.Trealm]*Queue),
		realms:    make([]sp.Trealm, 0),
		rr:        NewRealmRR(),
		qlen:      0,
		realmbins: chunkclnt.NewRealmBinPaths(),
	}
	pq.cond = sync.NewCond(&pq.mu)
	return pq
}

// XXX Deduplicate with lcsched
func (qd *QDir) GetProcs() []*proc.Proc {
	qd.pq.mu.Lock()
	defer qd.pq.mu.Unlock()

	procs := make([]*proc.Proc, 0, qd.pq.lenL())
	for _, q := range qd.pq.qs {
		for _, p := range q.pmap {
			procs = append(procs, p)
		}
	}
	return procs
}

// XXX Deduplicate with lcsched
func (qd *QDir) Lookup(pid string) (*proc.Proc, bool) {
	qd.pq.mu.Lock()
	defer qd.pq.mu.Unlock()

	for _, q := range qd.pq.qs {
		if p, ok := q.pmap[sp.Tpid(pid)]; ok {
			return p, ok
		}
	}
	return nil, false
}

// XXX Deduplicate with lcsched
func (pq *ProcQ) lenL() int {
	l := 0
	for _, q := range pq.qs {
		l += len(q.pmap)
	}
	return l
}

func (qd *QDir) Len() int {
	qd.pq.mu.Lock()
	defer qd.pq.mu.Unlock()

	return qd.pq.lenL()
}

func (pq *ProcQ) Enqueue(ctx fs.CtxI, req proto.EnqueueRequest, res *proto.EnqueueResponse) error {
	p := proc.NewProcFromProto(req.ProcProto)
	if p.GetRealm() != ctx.Principal().GetRealm() {
		return fmt.Errorf("Proc realm %v doesn't match principal realm %v", p.GetRealm(), ctx.Principal().GetRealm())
	}
	db.DPrintf(db.PROCQ, "[%v] Enqueue %v", p.GetRealm(), p)
	db.DPrintf(db.SPAWN_LAT, "[%v] RPC to procqsrv; time since spawn %v", p.GetPid(), time.Since(p.GetSpawnTime()))
	ch := make(chan *proc.ProcSeqno)
	pq.addProc(p, ch)
	db.DPrintf(db.PROCQ, "[%v] Enqueued %v", p.GetRealm(), p)
	seqno := <-ch
	res.ProcSeqno = seqno
	return nil
}

func (pq *ProcQ) addProc(p *proc.Proc, ch chan *proc.ProcSeqno) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Increase aggregate queue length.
	pq.qlen++
	// Increase the total number of procs spawned
	pq.tot.Add(1)
	// Get the queue for the realm.
	q := pq.getRealmQueue(p.GetRealm())
	// Enqueue the proc according to its realm.
	q.Enqueue(p, ch)
	// Note that the realm's queue is not empty
	pq.rr.RealmQueueNotEmpty(p.GetRealm())
	// Broadcast that a new proc may be runnable.
	pq.cond.Broadcast()
}

func (pq *ProcQ) replyToParent(pseqno *proc.ProcSeqno, p *proc.Proc, ch chan *proc.ProcSeqno, enqTS time.Time) {
	db.DPrintf(db.SPAWN_LAT, "[%v] Internal procqsrv Proc queueing time %v", p.GetPid(), time.Since(enqTS))
	db.DPrintf(db.PROCQ, "replyToParent child is on kid %v", pseqno.GetScheddID())
	ch <- pseqno
}

func (pq *ProcQ) GetStats(ctx fs.CtxI, req proto.GetStatsRequest, res *proto.GetStatsResponse) error {
	pq.realmMu.RLock()
	realms := make(map[string]int64, len(pq.realms))
	for _, r := range pq.realms {
		realms[string(r)] = 0
	}
	pq.realmMu.RUnlock()

	for r, _ := range realms {
		realms[r] = int64(pq.getRealmQueue(sp.Trealm(r)).Len())
	}
	res.Nqueued = realms

	return nil
}

func (pq *ProcQ) GetProc(ctx fs.CtxI, req proto.GetProcRequest, res *proto.GetProcResponse) error {
	db.DPrintf(db.PROCQ, "GetProc request by %v mem %v", req.KernelID, req.Mem)

	start := time.Now()
	// Try until we hit the timeout (which we may hit if the request is for too
	// few resources).
	for time.Since(start) < GET_PROC_TIMEOUT {
		pq.mu.Lock()
		// Get the next realm with procs queued, globally round-robin
		r, keepScanning := pq.rr.GetNextRealm(sp.NO_REALM)
		firstSeen := r
		for ; keepScanning; r, keepScanning = pq.rr.GetNextRealm(firstSeen) {
			q, ok := pq.qs[r]
			if !ok && r == sp.ROOTREALM {
				continue
			}
			db.DPrintf(db.PROCQ, "[%v] GetProc Try to dequeue %v", r, req.KernelID)
			p, ch, ts, ok := q.Dequeue(proc.Tmem(req.Mem), req.KernelID)
			db.DPrintf(db.PROCQ, "[%v] GetProc Done Try to dequeue %v", r, req.KernelID)
			if ok {
				if q.Len() == 0 {
					// Realm's queue is now empty
					pq.rr.RealmQueueEmpty(r)
				}
				// Decrease aggregate queue length.
				pq.qlen--
				db.DPrintf(db.PROCQ, "[%v] GetProc Dequeued for %v %v", r, req.KernelID, p)
				// Chunksrv relies on there only being one chunk server in the path to
				// avoid circular waits & deadlocks.
				if !chunksrv.IsChunkSrvPath(p.GetSigmaPath()[0]) {
					if kid, ok := pq.realmbins.GetBinKernelID(p.GetRealm(), p.GetProgram()); ok {
						p.PrependSigmaPath(chunk.ChunkdPath(kid))
					}
				}
				pq.realmbins.SetBinKernelID(p.GetRealm(), p.GetProgram(), req.KernelID)

				// Tell client about schedd chosen to run this proc. Do this
				// asynchronously so that schedd can proceed with the proc immediately.
				go pq.replyToParent(req.GetProcSeqno(), p, ch, ts)
				res.ProcProto = p.GetProto()
				res.OK = true
				res.QLen = uint32(pq.qlen)
				db.DPrintf(db.TEST, "assign %v BinKernelId %v to %v\n", p.GetPid(), p, req.KernelID)
				pq.mu.Unlock()
				return nil
			}
		}
		res.QLen = uint32(pq.qlen)
		// If unable to schedule a proc from any realm, wait.
		db.DPrintf(db.PROCQ, "GetProc No procs schedulable qs:%v", pq.qs)
		// Releases the lock, so we must re-acquire on the next loop iteration.
		ok := pq.waitOrTimeoutAndUnlock()
		// If timed out, respond to schedd to have it try another procq.
		if !ok {
			db.DPrintf(db.PROCQ, "Timed out GetProc request from: %v", req.KernelID)
			res.OK = false
			return nil
		}
		db.DPrintf(db.PROCQ, "Woke up GetProc request from: %v", req.KernelID)
	}
	res.OK = false
	return nil
}

func (pq *ProcQ) getRealmQueue(realm sp.Trealm) *Queue {
	pq.realmMu.RLock()
	defer pq.realmMu.RUnlock()

	q, ok := pq.tryGetRealmQueueL(realm)
	if !ok {
		// Promote to writer lock.
		pq.realmMu.RUnlock()
		pq.realmMu.Lock()
		// Check if the queue was created during lock promotion.
		q, ok = pq.tryGetRealmQueueL(realm)
		if !ok {
			// If the queue has still not been created, create it.
			q = newQueue()
			pq.qs[realm] = q
			// Don't add the root realm as a realm to choose to schedule from.
			if realm != sp.ROOTREALM {
				pq.realms = append(pq.realms, realm)
			}
		}
		// Demote to reader lock
		pq.realmMu.Unlock()
		pq.realmMu.RLock()
	}
	return q
}

// Caller must hold lock.
func (pq *ProcQ) tryGetRealmQueueL(realm sp.Trealm) (*Queue, bool) {
	q, ok := pq.qs[realm]
	return q, ok
}

func (pq *ProcQ) stats() {
	if !db.WillBePrinted(db.PROCQ) {
		return
	}
	for {
		time.Sleep(time.Second)
		// Increase the total number of procs spawned
		db.DPrintf(db.PROCQ, "Procq total size %v", pq.tot.Load())
	}
}

// Run a ProcQ
func Run() {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	sc.GetNetProxyClnt().AllowConnectionsFromAllRealms()
	pq := NewProcQ(sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.PROCQ, sc.ProcEnv().GetKernelID()), sc, pq)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	// export queued procs through procfs. maybe a subdir per realm?
	dir := procfs.NewProcDir(&QDir{pq})
	if err := ssrv.MkNod(sp.QUEUE, dir); err != nil {
		db.DFatalf("Error mknod %v: %v", sp.QUEUE, err)
	}
	// Perf monitoring
	p, err := perf.NewPerf(sc.ProcEnv(), perf.PROCQ)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()
	go pq.stats()
	ssrv.RunServer()
}
