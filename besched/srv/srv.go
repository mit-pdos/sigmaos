package srv

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"sigmaos/besched/proto"
	"sigmaos/chunk"
	"sigmaos/chunkclnt"
	"sigmaos/chunksrv"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/procfs"
	"sigmaos/schedqueue"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
)

const (
	GET_PROC_TIMEOUT = 50 * time.Millisecond
)

type BESched struct {
	mu          sync.Mutex
	realmMu     sync.RWMutex
	cond        *sync.Cond
	sc          *sigmaclnt.SigmaClnt
	qs          map[sp.Trealm]*schedqueue.Queue[*proc.ProcSeqno, chan *proc.ProcSeqno]
	realms      []sp.Trealm
	rr          *RealmRR
	qlen        int // Aggregate queue length, across all queues
	tot         atomic.Int64
	ngetprocReq atomic.Int64
	realmbins   *chunkclnt.RealmBinPaths
}

type QDir struct {
	be *BESched
}

func NewBESched(sc *sigmaclnt.SigmaClnt) *BESched {
	be := &BESched{
		sc:        sc,
		qs:        make(map[sp.Trealm]*schedqueue.Queue[*proc.ProcSeqno, chan *proc.ProcSeqno]),
		realms:    make([]sp.Trealm, 0),
		rr:        NewRealmRR(),
		qlen:      0,
		realmbins: chunkclnt.NewRealmBinPaths(),
	}
	be.cond = sync.NewCond(&be.mu)
	return be
}

// XXX Deduplicate with lcsched
func (qd *QDir) GetProcs() []*proc.Proc {
	qd.be.mu.Lock()
	defer qd.be.mu.Unlock()

	procs := make([]*proc.Proc, 0, qd.be.lenL())
	for _, q := range qd.be.qs {
		pmap := q.GetPMapL()
		for _, p := range pmap {
			procs = append(procs, p)
		}
	}
	return procs
}

// XXX Deduplicate with lcsched
func (qd *QDir) Lookup(pid string) (*proc.Proc, bool) {
	qd.be.mu.Lock()
	defer qd.be.mu.Unlock()

	for _, q := range qd.be.qs {
		pmap := q.GetPMapL()
		if p, ok := pmap[sp.Tpid(pid)]; ok {
			return p, ok
		}
	}
	return nil, false
}

// XXX Deduplicate with lcsched
func (be *BESched) lenL() int {
	l := 0
	for _, q := range be.qs {
		l += q.Len()
	}
	return l
}

func (qd *QDir) Len() int {
	qd.be.mu.Lock()
	defer qd.be.mu.Unlock()

	return qd.be.lenL()
}

func (be *BESched) Enqueue(ctx fs.CtxI, req proto.EnqueueRequest, res *proto.EnqueueResponse) error {
	p := proc.NewProcFromProto(req.ProcProto)
	if p.GetRealm() != ctx.Principal().GetRealm() {
		return fmt.Errorf("Proc realm %v doesn't match principal realm %v", p.GetRealm(), ctx.Principal().GetRealm())
	}
	db.DPrintf(db.BESCHED, "[%v] Enqueue %v", p.GetRealm(), p)
	db.DPrintf(db.SPAWN_LAT, "[%v] RPC to beschedsrv; time since spawn %v", p.GetPid(), time.Since(p.GetSpawnTime()))
	ch := make(chan *proc.ProcSeqno)
	be.addProc(p, ch)
	db.DPrintf(db.BESCHED, "[%v] Enqueued %v", p.GetRealm(), p)
	seqno := <-ch
	res.ProcSeqno = seqno
	return nil
}

func (be *BESched) addProc(p *proc.Proc, ch chan *proc.ProcSeqno) {
	lockStart := time.Now()

	be.mu.Lock()
	defer be.mu.Unlock()

	db.DPrintf(db.SPAWN_LAT, "Time to acquire lock in addProc: %v", time.Since(lockStart))

	// Increase aggregate queue length.
	be.qlen++
	// Increase the total number of procs spawned
	be.tot.Add(1)
	// Get the queue for the realm.
	q := be.getRealmQueue(p.GetRealm())
	// Enqueue the proc according to its realm.
	q.Enqueue(p, ch)
	// Note that the realm's queue is not empty
	be.rr.RealmQueueNotEmpty(p.GetRealm())
	// Broadcast that a new proc may be runnable.
	be.cond.Broadcast()
}

func (be *BESched) replyToParent(pseqno *proc.ProcSeqno, p *proc.Proc, ch chan *proc.ProcSeqno, enqTS time.Time) {
	db.DPrintf(db.SPAWN_LAT, "[%v] Internal beschedsrv Proc queueing time %v", p.GetPid(), time.Since(enqTS))
	db.DPrintf(db.BESCHED, "replyToParent child is on kid %v", pseqno.GetScheddID())
	ch <- pseqno
}

func (be *BESched) GetStats(ctx fs.CtxI, req proto.GetStatsRequest, res *proto.GetStatsResponse) error {
	be.realmMu.RLock()
	realms := make(map[string]int64, len(be.realms))
	for _, r := range be.realms {
		realms[string(r)] = 0
	}
	be.realmMu.RUnlock()

	for r, _ := range realms {
		realms[r] = int64(be.getRealmQueue(sp.Trealm(r)).Len())
	}
	res.Nqueued = realms

	return nil
}

func (be *BESched) GetProc(ctx fs.CtxI, req proto.GetProcRequest, res *proto.GetProcResponse) error {
	db.DPrintf(db.BESCHED, "GetProc request by %v mem %v", req.KernelID, req.Mem)

	be.ngetprocReq.Add(1)

	start := time.Now()
	// Try until we hit the timeout (which we may hit if the request is for too
	// few resources).
	for time.Since(start) < GET_PROC_TIMEOUT {
		lockStart := time.Now()
		be.mu.Lock()
		lockDur := time.Since(lockStart)
		db.DPrintf(db.SPAWN_LAT, "Time to acquire lock in GetProc: %v", lockDur)
		scanStart := time.Now()
		// Get the next realm with procs queued, globally round-robin
		r, keepScanning := be.rr.GetNextRealm(sp.NO_REALM)
		firstSeen := r
		for ; keepScanning; r, keepScanning = be.rr.GetNextRealm(firstSeen) {
			q, ok := be.qs[r]
			if !ok && r == sp.ROOTREALM {
				continue
			}
			db.DPrintf(db.BESCHED, "[%v] GetProc Try to dequeue %v", r, req.KernelID)
			dequeueStart := time.Now()
			p, ch, ts, ok := q.Dequeue(func(p *proc.Proc) bool {
				return isEligible(p, proc.Tmem(req.Mem), req.KernelID)
			})
			dequeueDur := time.Since(dequeueStart)
			db.DPrintf(db.BESCHED, "[%v] GetProc Done Try to dequeue %v", r, req.KernelID)
			if ok {
				scanDur := time.Since(scanStart)
				db.DPrintf(db.SPAWN_LAT, "[%v] Queue scan time: %v dequeue time %v lock time %v", p.GetPid(), scanDur, dequeueDur, lockDur)
				postDequeueStart := time.Now()
				if q.Len() == 0 {
					// Realm's queue is now empty
					be.rr.RealmQueueEmpty(r)
				}
				// Decrease aggregate queue length.
				be.qlen--
				db.DPrintf(db.BESCHED, "[%v] GetProc Dequeued for %v %v", r, req.KernelID, p)
				// Chunksrv relies on there only being one chunk server in the path to
				// avoid circular waits & deadlocks.
				if !chunksrv.IsChunkSrvPath(p.GetSigmaPath()[0]) {
					if kid, ok := be.realmbins.GetBinKernelID(p.GetRealm(), p.GetProgram()); ok {
						p.PrependSigmaPath(chunk.ChunkdPath(kid))
					}
				}
				be.realmbins.SetBinKernelID(p.GetRealm(), p.GetProgram(), req.KernelID)

				// Tell client about schedd chosen to run this proc. Do this
				// asynchronously so that schedd can proceed with the proc immediately.
				go be.replyToParent(req.GetProcSeqno(), p, ch, ts)
				res.ProcProto = p.GetProto()
				res.OK = true
				res.QLen = uint32(be.qlen)
				db.DPrintf(db.SPAWN_LAT, "[%v] Post-dequeue time: %v Queue scan time %v dequeue time %v lock time %v", p.GetPid(), time.Since(postDequeueStart), scanDur, dequeueDur, lockDur)
				db.DPrintf(db.BESCHED, "assign %v BinKernelId %v to %v\n", p.GetPid(), p, req.KernelID)
				be.mu.Unlock()
				return nil
			}
		}
		res.QLen = uint32(be.qlen)
		// If unable to schedule a proc from any realm, wait.
		db.DPrintf(db.BESCHED, "GetProc No procs schedulable qs:%v", be.qs)
		// Releases the lock, so we must re-acquire on the next loop iteration.
		ok := be.waitOrTimeoutAndUnlock()
		// If timed out, respond to schedd to have it try another besched.
		if !ok {
			db.DPrintf(db.BESCHED, "Timed out GetProc request from: %v", req.KernelID)
			res.OK = false
			return nil
		}
		db.DPrintf(db.BESCHED, "Woke up GetProc request from: %v", req.KernelID)
	}
	res.OK = false
	return nil
}

func isEligible(p *proc.Proc, mem proc.Tmem, scheddID string) bool {
	if p.GetMem() > mem {
		return false
	}
	if p.HasNoKernelPref() {
		return true
	}
	return p.HasKernelPref(scheddID)
}

func (be *BESched) getRealmQueue(realm sp.Trealm) *schedqueue.Queue[*proc.ProcSeqno, chan *proc.ProcSeqno] {
	be.realmMu.RLock()
	defer be.realmMu.RUnlock()

	q, ok := be.tryGetRealmQueueL(realm)
	if !ok {
		// Promote to writer lock.
		be.realmMu.RUnlock()
		be.realmMu.Lock()
		// Check if the queue was created during lock promotion.
		q, ok = be.tryGetRealmQueueL(realm)
		if !ok {
			// If the queue has still not been created, create it.
			q = schedqueue.NewQueue[*proc.ProcSeqno, chan *proc.ProcSeqno]()
			be.qs[realm] = q
			// Don't add the root realm as a realm to choose to schedule from.
			if realm != sp.ROOTREALM {
				be.realms = append(be.realms, realm)
			}
		}
		// Demote to reader lock
		be.realmMu.Unlock()
		be.realmMu.RLock()
	}
	return q
}

// Caller must hold lock.
func (be *BESched) tryGetRealmQueueL(realm sp.Trealm) (*schedqueue.Queue[*proc.ProcSeqno, chan *proc.ProcSeqno], bool) {
	q, ok := be.qs[realm]
	return q, ok
}

func (be *BESched) stats() {
	if !db.WillBePrinted(db.BESCHED) {
		return
	}
	for {
		time.Sleep(time.Second)
		// Increase the total number of procs spawned
		db.DPrintf(db.BESCHED, "Procq total size %v", be.tot.Load())
	}
}

func (be *BESched) getprocStats() {
	if !db.WillBePrinted(db.SPAWN_LAT) {
		return
	}
	for {
		ngp1 := be.ngetprocReq.Load()
		time.Sleep(5 * time.Second)
		ngp2 := be.ngetprocReq.Load()
		db.DPrintf(db.SPAWN_LAT, "Stats ngetproc: %v/s", (ngp2-ngp1)/5)
	}
}

// Run a BESched
func Run() {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	sc.GetNetProxyClnt().AllowConnectionsFromAllRealms()
	be := NewBESched(sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.BESCHED, sc.ProcEnv().GetKernelID()), sc, be)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	// export queued procs through procfs. maybe a subdir per realm?
	dir := procfs.NewProcDir(&QDir{be})
	if err := ssrv.MkNod(sp.QUEUE, dir); err != nil {
		db.DFatalf("Error mknod %v: %v", sp.QUEUE, err)
	}
	// Perf monitoring
	p, err := perf.NewPerf(sc.ProcEnv(), perf.BESCHED)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()
	go be.stats()
	go be.getprocStats()
	ssrv.RunServer()
}
