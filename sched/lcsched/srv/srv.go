package srv

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rpc"
	beschedproto "sigmaos/sched/besched/proto"
	"sigmaos/sched/lcsched/proto"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proc/chunk"
	chunkclnt "sigmaos/sched/msched/proc/chunk/clnt"
	"sigmaos/sched/queue"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib/dirwatcher"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
)

type LCSched struct {
	mu         sync.Mutex
	cond       *sync.Cond
	sc         *sigmaclnt.SigmaClnt
	mschedclnt *mschedclnt.MSchedClnt
	qs         map[sp.Trealm]*queue.Queue[string, chan string]
	mscheds    map[string]*Resources
	realmbins  *chunkclnt.RealmBinPaths
}

func NewLCSched(sc *sigmaclnt.SigmaClnt) *LCSched {
	lcs := &LCSched{
		sc:         sc,
		mschedclnt: mschedclnt.NewMSchedClnt(sc.FsLib, sp.NOT_SET),
		qs:         make(map[sp.Trealm]*queue.Queue[string, chan string]),
		mscheds:    make(map[string]*Resources),
		realmbins:  chunkclnt.NewRealmBinPaths(),
	}
	lcs.cond = sync.NewCond(&lcs.mu)
	return lcs
}

func (lcs *LCSched) Enqueue(ctx fs.CtxI, req beschedproto.EnqueueReq, res *beschedproto.EnqueueRep) error {
	start := time.Now()
	p := proc.NewProcFromProto(req.ProcProto)
	if p.GetRealm() != ctx.Principal().GetRealm() {
		return fmt.Errorf("Proc realm %v doesn't match principal realm %v", p.GetRealm(), ctx.Principal().GetRealm())
	}
	perf.LogSpawnLatency("LCSched.Enqueue recvd proc", p.GetPid(), p.GetSpawnTime(), perf.TIME_NOT_SET)
	defer func(start time.Time) {
		perf.LogSpawnLatency("LCSched.Eneuque E2e", p.GetPid(), p.GetSpawnTime(), start)
	}(start)
	db.DPrintf(db.LCSCHED, "[%v] Enqueued %v", p.GetRealm(), p)

	ch := make(chan string)
	start = time.Now()
	lcs.addProc(p, ch)
	perf.LogSpawnLatency("LCSched.addProc", p.GetPid(), p.GetSpawnTime(), start)
	res.MSchedID = <-ch
	return nil
}

func (lcs *LCSched) RegisterMSched(ctx fs.CtxI, req proto.RegisterMSchedReq, res *proto.RegisterMSchedRep) error {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	db.DPrintf(db.LCSCHED, "Register MSched id:%v mcpu:%v mem:%v", req.KernelID, req.McpuInt, req.MemInt)
	if _, ok := lcs.mscheds[req.KernelID]; ok {
		db.DPrintf(db.ERROR, "Double-register msched %v", req.KernelID)
		return fmt.Errorf("Double-register msched %v", req.KernelID)
	}
	lcs.mscheds[req.KernelID] = newResources(req.McpuInt, req.MemInt)
	lcs.cond.Broadcast()
	// Immediately mount the msched RPC file
	ep := sp.NewEndpointFromProto(req.EndpointProto)
	pn := filepath.Join(sp.MSCHED, req.KernelID, rpc.RPC)
	db.DPrintf(db.LCSCHED, "Mount[%v] %v as %v", ep, rpc.RPC, pn)
	err := lcs.sc.FsLib.MountTree(ep, rpc.RPC, pn)
	if err != nil {
		db.DPrintf(db.ERROR, "Err MountTree: ep %v err %v", ep, err)
		return err
	}

	// Monitor the msched
	go lcs.monitorMSched(req.KernelID)
	return nil
}

func (lcs *LCSched) monitorMSched(kernelID string) {
	// Wait for the msched to be removed
	if err := dirwatcher.WaitRemove(lcs.sc.FsLib, filepath.Join(sp.MSCHED, kernelID)); err != nil {
		db.DPrintf(db.ERROR, "WaitRemove msched %v", kernelID)
	}

	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	db.DPrintf(db.LCSCHED, "Deregister MSched %v", kernelID)

	// Deregister the msched
	delete(lcs.mscheds, kernelID)
}

func (lcs *LCSched) schedule() {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	// Keep scheduling forever.
	for {
		var success bool
		for realm, q := range lcs.qs {
			db.DPrintf(db.LCSCHED, "Try to schedule realm %v", realm)
			for kid, r := range lcs.mscheds {
				p, ch, enqueueT, ok := q.Dequeue(func(p *proc.Proc) bool {
					return isEligible(p, r.mcpu, r.mem)
				})
				if ok {
					db.DPrintf(db.LCSCHED, "Successfully schedule realm %v", realm)
					perf.LogSpawnLatency("LCSched.Dequeue found eligible msched", p.GetPid(), p.GetSpawnTime(), enqueueT)
					// Alloc resources for the proc
					r.alloc(p)
					go lcs.runProc(kid, p, ch, r)
					success = true
					// Move on to the next realm
					break
				}
			}
			db.DPrintf(db.LCSCHED, "Done trying to schedule realm %v success %v", realm, success)
		}
		// If scheduling was unsuccessful, wait.
		if !success {
			db.DPrintf(db.LCSCHED, "Schedule wait")
			lcs.cond.Wait()
		}
		db.DPrintf(db.LCSCHED, "Schedule retry")
	}
}

func (lcs *LCSched) runProc(kernelID string, p *proc.Proc, ch chan string, r *Resources) {
	// Chunksrv relies on there only being one chunk server in the path to
	// avoid circular waits & deadlocks.
	if !chunk.IsChunkSrvPath(p.GetSigmaPath()[0]) {
		if kid, ok := lcs.realmbins.GetBinKernelID(p.GetRealm(), p.GetProgram()); ok {
			p.PrependSigmaPath(chunk.ChunkdPath(kid))
		}
	}
	lcs.realmbins.SetBinKernelID(p.GetRealm(), p.GetProgram(), kernelID)
	db.DPrintf(db.LCSCHED, "runProc kernelID %v p %v", kernelID, p)
	start := time.Now()
	if err := lcs.mschedclnt.ForceRun(kernelID, false, p); err != nil {
		db.DPrintf(db.ALWAYS, "MSched.Run %v err %v", kernelID, err)
		// Re-enqueue the proc
		lcs.addProc(p, ch)
		return
	}
	perf.LogSpawnLatency("LCSched.runProc ForceRun", p.GetPid(), p.GetSpawnTime(), start)
	// Notify the spawner that a msched has been chosen.
	ch <- kernelID
	// Wait for the proc to exit.
	lcs.waitProcExit(kernelID, p, r)
}

func (lcs *LCSched) waitProcExit(kernelID string, p *proc.Proc, r *Resources) {
	// RPC the msched this proc was spawned on to wait for the proc to exit.
	db.DPrintf(db.LCSCHED, "WaitExit %v RPC", p.GetPid())
	if _, err := lcs.mschedclnt.Wait(mschedclnt.EXIT, kernelID, proc.NewProcSeqno(sp.NOT_SET, kernelID, 0, 0), p.GetPid()); err != nil {
		db.DPrintf(db.ALWAYS, "Error MSched WaitExit: %v", err)
	}
	db.DPrintf(db.LCSCHED, "Proc exited %v", p.GetPid())
	// Lock to modify resource allocations
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	r.free(p)
	// Notify that more resources have become available (and thus procs may now
	// be schedulable).
	lcs.cond.Broadcast()
}

func (lcs *LCSched) addProc(p *proc.Proc, ch chan string) {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	q, ok := lcs.qs[p.GetRealm()]
	if !ok {
		q = lcs.addRealmQueueL(p.GetRealm())
	}
	// Enqueue the proc according to its realm
	q.Enqueue(p, ch)
	// Signal that a new proc may be runnable.
	lcs.cond.Signal()
}

func isEligible(p *proc.Proc, mcpu proc.Tmcpu, mem proc.Tmem) bool {
	if p.GetMem() <= mem && p.GetMcpu() <= mcpu {
		return true
	}
	return false
}

// Caller must hold lock.
func (lcs *LCSched) addRealmQueueL(realm sp.Trealm) *queue.Queue[string, chan string] {
	q := queue.NewQueue[string, chan string]()
	lcs.qs[realm] = q
	return q
}

// Run an LCSched
func Run() {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	sc.GetDialProxyClnt().AllowConnectionsFromAllRealms()
	lcs := NewLCSched(sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.LCSCHED, sc.ProcEnv().GetPID().String()), sc, lcs)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}

	crash.Failer(sc.FsLib, crash.LCSCHED_CRASH, func(e crash.Tevent) {
		crash.Crash()
	})

	// Perf monitoring
	p, err := perf.NewPerf(sc.ProcEnv(), perf.LCSCHED)

	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}

	defer p.Done()
	go lcs.schedule()

	ssrv.RunServer()
}
