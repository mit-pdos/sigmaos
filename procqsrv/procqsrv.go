package procqsrv

import (
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	proto "sigmaos/procqsrv/proto"
	"sigmaos/scheddclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

const (
	GET_PROC_TIMEOUT = 500 * time.Millisecond
)

type ProcQ struct {
	mu         sync.Mutex
	realmMu    sync.RWMutex
	cond       *sync.Cond
	mfs        *memfssrv.MemFs
	scheddclnt *scheddclnt.ScheddClnt
	qs         map[sp.Trealm]*Queue
	qlen       int // Aggregate queue length, across all queues
}

func NewProcQ(mfs *memfssrv.MemFs) *ProcQ {
	pq := &ProcQ{
		mfs:        mfs,
		scheddclnt: scheddclnt.NewScheddClnt(mfs.SigmaClnt().FsLib),
		qs:         make(map[sp.Trealm]*Queue),
		qlen:       0,
	}
	pq.cond = sync.NewCond(&pq.mu)
	return pq
}

func (pq *ProcQ) Enqueue(ctx fs.CtxI, req proto.EnqueueRequest, res *proto.EnqueueResponse) error {
	p := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.PROCQ, "[%v] Enqueue %v", p.GetRealm(), p)
	db.DPrintf(db.SPAWN_LAT, "[%v] RPC to procqsrv time %v", p.GetPid(), time.Since(p.GetSpawnTime()))
	ch := pq.addProc(p)
	db.DPrintf(db.PROCQ, "[%v] Enqueued %v", p.GetRealm(), p)
	res.KernelID = <-ch
	return nil
}

func (pq *ProcQ) addProc(p *proc.Proc) chan string {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Increase aggregate queue length.
	pq.qlen++
	// Get the queue for the realm.
	q := pq.getRealmQueue(p.GetRealm())
	// Enqueue the proc according to its realm.
	ch := q.Enqueue(p)
	// Broadcast that a new proc may be runnable.
	pq.cond.Broadcast()
	return ch
}

func (pq *ProcQ) runProc(kernelID string, p *proc.Proc, ch chan string, enqTS time.Time) {
	db.DPrintf(db.SPAWN_LAT, "[%v] Internal procqsrv Proc queueing time %v", p.GetPid(), time.Since(enqTS))
	// Must push the proc to the schedd before responding to the parent because
	// we must guarantee that the schedd knows about it before talking to the
	// parent. Otherwise, the response to the parent could arrive first and the
	// parent could ask schedd about the proc before the schedd learns about the
	// proc.
	if err := pq.scheddclnt.ForceRun(kernelID, p); err != nil {
		db.DFatalf("Error ForceRun proc: %v", err)
	}
	ch <- kernelID
}

func (pq *ProcQ) GetProc(ctx fs.CtxI, req proto.GetProcRequest, res *proto.GetProcResponse) error {
	db.DPrintf(db.PROCQ, "GetProc request by %v", req.KernelID)

	for {
		pq.mu.Lock()
		// Iterate through the realms round-robin.
		for r, q := range pq.qs {
			p, ch, ts, ok := q.Dequeue()
			if ok {
				// Decrease aggregate queue length.
				pq.qlen--
				db.DPrintf(db.PROCQ, "[%v] Dequeued for %v %v", r, req.KernelID, p)
				// Push proc to schedd. Do this asynchronously so we don't hold locks
				// across RPCs.
				go pq.runProc(req.KernelID, p, ch, ts)
				res.OK = true
				pq.mu.Unlock()
				return nil
			}
		}
		// If unable to schedule a proc from any realm, wait.
		db.DPrintf(db.PROCQ, "No procs schedulable qs:%v", pq.qs)
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

// Run a ProcQ
func Run() {
	pcfg := proc.GetProcEnv()
	mfs, err := memfssrv.NewMemFs(path.Join(sp.PROCQ, pcfg.GetPID().String()), pcfg)
	if err != nil {
		db.DFatalf("Error NewMemFs: %v", err)
	}
	pq := NewProcQ(mfs)
	ssrv, err := sigmasrv.NewSigmaSrvMemFs(mfs, pq)
	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}
	setupMemFsSrv(ssrv.MemFs)
	setupFs(ssrv.MemFs)
	// Perf monitoring
	p, err := perf.NewPerf(pcfg, perf.PROCQ)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()

	ssrv.RunServer()
}
