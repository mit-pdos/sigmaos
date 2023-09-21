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
	cond       *sync.Cond
	mfs        *memfssrv.MemFs
	scheddclnt *scheddclnt.ScheddClnt
	qs         map[sp.Trealm]*Queue
}

func NewProcQ(mfs *memfssrv.MemFs) *ProcQ {
	pq := &ProcQ{
		mfs:        mfs,
		scheddclnt: scheddclnt.NewScheddClnt(mfs.SigmaClnt().FsLib),
		qs:         make(map[sp.Trealm]*Queue),
	}
	pq.cond = sync.NewCond(&pq.mu)
	return pq
}

func (pq *ProcQ) Enqueue(ctx fs.CtxI, req proto.EnqueueRequest, res *proto.EnqueueResponse) error {
	p := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.PROCQ, "[%v] Enqueue %v", p.GetRealm(), p)
	ch := pq.addProc(p)
	db.DPrintf(db.PROCQ, "[%v] Enqueued %v", p.GetRealm(), p)
	res.KernelID = <-ch
	return nil
}

func (pq *ProcQ) addProc(p *proc.Proc) chan string {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	q, ok := pq.qs[p.GetRealm()]
	if !ok {
		q = pq.addRealmQueueL(p.GetRealm())
	}
	// Enqueue the proc according to its realm
	ch := q.Enqueue(p)
	// Broadcast that a new proc may be runnable.
	pq.cond.Broadcast()
	return ch
}

func (pq *ProcQ) runProc(kernelID string, p *proc.Proc, ch chan string) {
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

	pq.mu.Lock()
	defer pq.mu.Unlock()

	db.DPrintf(db.PROCQ, "GetProc acquired lock for %v", req.KernelID)

	// XXX seems fishy to loop forever...
	for {
		// XXX Should probably do this more efficiently (just select a realm).
		// Iterate through the realms round-robin.
		for r, q := range pq.qs {
			p, ch, ok := q.Dequeue()
			if ok {
				db.DPrintf(db.PROCQ, "[%v] Dequeued for %v %v", r, req.KernelID, p)
				// Push proc to schedd. Do this asynchronously so we don't hold locks
				// across RPCs.
				go pq.runProc(req.KernelID, p, ch)
				res.OK = true
				return nil
			}
		}
		// If unable to schedule a proc from any realm, wait.
		db.DPrintf(db.PROCQ, "No procs schedulable qs:%v", pq.qs)
		ok := pq.waitOrTimeoutL()
		// If timed out, respond to schedd to have it try another procq.
		if !ok {
			res.OK = false
			return nil
		}
	}
	return nil
}

// Caller must hold lock.
func (pq *ProcQ) addRealmQueueL(realm sp.Trealm) *Queue {
	q := newQueue()
	pq.qs[realm] = q
	return q
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
