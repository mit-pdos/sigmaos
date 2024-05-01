package procqsrv

import (
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	"sigmaos/chunk"
	"sigmaos/chunkclnt"
	"sigmaos/chunksrv"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/keys"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procfs"
	proto "sigmaos/procqsrv/proto"
	"sigmaos/rand"
	"sigmaos/scheddclnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

const (
	GET_PROC_TIMEOUT = 50 * time.Millisecond
)

type ProcQ struct {
	mu         sync.Mutex
	realmMu    sync.RWMutex
	cond       *sync.Cond
	sc         *sigmaclnt.SigmaClnt
	scheddclnt *scheddclnt.ScheddClnt
	qs         map[sp.Trealm]*Queue
	realms     []sp.Trealm
	qlen       int // Aggregate queue length, across all queues
	tot        atomic.Int64
	realmbins  *chunkclnt.RealmBinPaths
}

type QDir struct {
	pq *ProcQ
}

func NewProcQ(sc *sigmaclnt.SigmaClnt) *ProcQ {
	pq := &ProcQ{
		sc:         sc,
		scheddclnt: scheddclnt.NewScheddClnt(sc.FsLib),
		qs:         make(map[sp.Trealm]*Queue),
		realms:     make([]sp.Trealm, 0),
		qlen:       0,
		realmbins:  chunkclnt.NewRealmBinPaths(),
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
	db.DPrintf(db.PROCQ, "[%v] Enqueue %v", p.GetRealm(), p)
	db.DPrintf(db.SPAWN_LAT, "[%v] RPC to procqsrv time %v", p.GetPid(), time.Since(p.GetSpawnTime()))
	ch := make(chan string)
	pq.addProc(p, ch)
	db.DPrintf(db.PROCQ, "[%v] Enqueued %v", p.GetRealm(), p)
	res.KernelID = <-ch
	return nil
}

func (pq *ProcQ) addProc(p *proc.Proc, kidch chan string) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Increase aggregate queue length.
	pq.qlen++
	// Increase the total number of procs spawned
	pq.tot.Add(1)
	// Get the queue for the realm.
	q := pq.getRealmQueue(p.GetRealm())
	// Enqueue the proc according to its realm.
	q.Enqueue(p, kidch)
	// Broadcast that a new proc may be runnable.
	pq.cond.Broadcast()
}

func (pq *ProcQ) runProc(kernelID string, p *proc.Proc, ch chan string, enqTS time.Time) {
	db.DPrintf(db.SPAWN_LAT, "[%v] Internal procqsrv Proc queueing time %v", p.GetPid(), time.Since(enqTS))
	db.DPrintf(db.PROCQ, "runProc on kid %v", kernelID)
	// Must push the proc to the schedd before responding to the parent because
	// we must guarantee that the schedd knows about it before talking to the
	// parent. Otherwise, the response to the parent could arrive first and the
	// parent could ask schedd about the proc before the schedd learns about the
	// proc.
	if err := pq.scheddclnt.ForceRun(kernelID, true, p); err != nil {
		db.DPrintf(db.PROCQ_ERR, "Error ForceRun proc on kid %v: %v", kernelID, err)
		pq.addProc(p, ch)
		if serr.IsErrorUnavailable(err) {
			pq.realmbins.DelBinKernelID(p.GetRealm(), p.GetProgram(), kernelID)
		}
		return
	}
	db.DPrintf(db.PROCQ, "Done runProc on kid %v", kernelID)
	ch <- kernelID
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

	// Pick a random realm to start with.
	rOff := int(rand.Int64(9999))
	start := time.Now()
	// Try until we hit the timeout (which we may hit if the request is for too
	// few resources).
	for time.Since(start) < GET_PROC_TIMEOUT {
		pq.mu.Lock()
		nrealm := len(pq.realms)
		// Iterate through the realms round-robin, starting with the random index.
		first := ""
		// +1 to include the root realm, which isn't part of the realms slice.
		for i := 0; i < nrealm+1; i++ {
			var r sp.Trealm
			// If we tried to schedule every other realm, then try to schedule the
			// root realm. We do it this way to avoid biasing scheduling in favor of
			// realms that start up earlier. In normal operation there will be
			// nothing to spawn for the root realm, and this would mean that a random
			// choice of index into the pq.realms slice would give the first non-root
			// realm an extra shot at being scheduled (since we move round-robin
			// after the random choice).
			if i == nrealm {
				// Only try the root realm once we've exhausted all other options.
				r = sp.ROOTREALM
			} else {
				r = pq.realms[(rOff+i)%len(pq.realms)]
			}
			q, ok := pq.qs[r]
			if !ok && r == sp.ROOTREALM {
				continue
			}
			if first == "" {
				first = r.String()
				db.DPrintf(db.PROCQ, "First try to dequeue from %v", r)
			}
			db.DPrintf(db.PROCQ, "[%v] GetProc Try to dequeue %v", r, req.KernelID)
			p, ch, ts, ok := q.Dequeue(proc.Tmem(req.Mem), req.KernelID)
			db.DPrintf(db.PROCQ, "[%v] GetProc Done Try to dequeue %v", r, req.KernelID)
			if ok {
				// Decrease aggregate queue length.
				pq.qlen--
				db.DPrintf(db.PROCQ, "[%v] GetProc Dequeued for %v %v", r, req.KernelID, p)
				if !chunksrv.IsChunkSrvPath(p.GetSigmaPath()[0]) {
					if kid, ok := pq.realmbins.GetBinKernelID(p.GetRealm(), p.GetProgram()); ok {
						p.PrependSigmaPath(chunk.ChunkdPath(kid))
					}
				}
				pq.realmbins.SetBinKernelID(p.GetRealm(), p.GetProgram(), req.KernelID)

				// Push proc to schedd. Do this asynchronously so we don't hold locks
				// across RPCs.
				go pq.runProc(req.KernelID, p, ch, ts)
				res.OK = true
				// Note the amount of memory required by the proc, so that schedd can
				// do resource accounting and (potentially) temper future GetProc
				// requests.
				res.Mem = uint32(p.GetMem())
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
func Run(masterPubKey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sc),
		masterPubKey,
		nil,
		sp.Tsigner(pe.GetPID()),
		pubkey,
		privkey,
	)
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(pe.GetPID()), sp.NOT_SET, kmgr)
	if err != nil {
		db.DFatalf("Error NewAuthSrv %v", err)
	}
	sc.SetAuthSrv(as)
	pq := NewProcQ(sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(path.Join(sp.PROCQ, sc.ProcEnv().GetKernelID()), sc, pq)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	// export queued procs through procfs. maybe a subdir per realm?
	dir := procfs.NewProcDir(&QDir{pq})
	if err := ssrv.MkNod(sp.QUEUE, dir); err != nil {
		db.DFatalf("Error mknod %v: %v", sp.QUEUE, err)
	}
	// Update schedds, so that unionrpcclnt starts monitoring them
	pq.scheddclnt.UpdateSchedds()
	// Perf monitoring
	p, err := perf.NewPerf(sc.ProcEnv(), perf.PROCQ)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()
	go pq.stats()
	ssrv.RunServer()
}
