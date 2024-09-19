package lcschedsrv

import (
	"fmt"
	"path/filepath"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	proto "sigmaos/lcschedsrv/proto"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procfs"
	pqproto "sigmaos/procqsrv/proto"
	"sigmaos/scheddclnt"
	"sigmaos/schedqueue"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type LCSched struct {
	mu         sync.Mutex
	cond       *sync.Cond
	sc         *sigmaclnt.SigmaClnt
	scheddclnt *scheddclnt.ScheddClnt
	qs         map[sp.Trealm]*schedqueue.Queue[string, chan string]
	schedds    map[string]*Resources
}

type QDir struct {
	lcs *LCSched
}

func NewLCSched(sc *sigmaclnt.SigmaClnt) *LCSched {
	lcs := &LCSched{
		sc:         sc,
		scheddclnt: scheddclnt.NewScheddClnt(sc.FsLib),
		qs:         make(map[sp.Trealm]*schedqueue.Queue[string, chan string]),
		schedds:    make(map[string]*Resources),
	}
	lcs.cond = sync.NewCond(&lcs.mu)
	return lcs
}

func (qd *QDir) GetProcs() []*proc.Proc {
	qd.lcs.mu.Lock()
	defer qd.lcs.mu.Unlock()

	procs := make([]*proc.Proc, 0, qd.lcs.lenL())
	for _, q := range qd.lcs.qs {
		pmap := q.GetPMapL()
		for _, p := range pmap {
			procs = append(procs, p)
		}
	}
	return procs
}

func (qd *QDir) Lookup(pid string) (*proc.Proc, bool) {
	qd.lcs.mu.Lock()
	defer qd.lcs.mu.Unlock()

	for _, q := range qd.lcs.qs {
		pmap := q.GetPMapL()
		if p, ok := pmap[sp.Tpid(pid)]; ok {
			return p, ok
		}
	}
	return nil, false
}

func (lcs *LCSched) lenL() int {
	l := 0
	for _, q := range lcs.qs {
		l += q.Len()
	}
	return l
}

func (qd *QDir) Len() int {
	qd.lcs.mu.Lock()
	defer qd.lcs.mu.Unlock()

	return qd.lcs.lenL()
}

func (lcs *LCSched) Enqueue(ctx fs.CtxI, req pqproto.EnqueueRequest, res *pqproto.EnqueueResponse) error {
	p := proc.NewProcFromProto(req.ProcProto)
	if p.GetRealm() != ctx.Principal().GetRealm() {
		return fmt.Errorf("Proc realm %v doesn't match principal realm %v", p.GetRealm(), ctx.Principal().GetRealm())
	}
	db.DPrintf(db.LCSCHED, "[%v] Enqueued %v", p.GetRealm(), p)

	ch := make(chan string)
	lcs.addProc(p, ch)
	res.ScheddID = <-ch
	return nil
}

func (lcs *LCSched) RegisterSchedd(ctx fs.CtxI, req proto.RegisterScheddRequest, res *proto.RegisterScheddResponse) error {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	db.DPrintf(db.LCSCHED, "Register Schedd id:%v mcpu:%v mem:%v", req.KernelID, req.McpuInt, req.MemInt)
	if _, ok := lcs.schedds[req.KernelID]; ok {
		db.DPrintf(db.ERROR, "Double-register schedd %v", req.KernelID)
		return fmt.Errorf("Double-register schedd %v", req.KernelID)
	}
	lcs.schedds[req.KernelID] = newResources(req.McpuInt, req.MemInt)
	lcs.cond.Broadcast()
	return nil
}

func (lcs *LCSched) schedule() {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	// Keep scheduling forever.
	for {
		var success bool
		for realm, q := range lcs.qs {
			db.DPrintf(db.LCSCHED, "Try to schedule realm %v", realm)
			for kid, r := range lcs.schedds {
				p, ch, _, ok := q.Dequeue(func(p *proc.Proc) bool {
					return isEligible(p, r.mcpu, r.mem)
				})
				if ok {
					db.DPrintf(db.LCSCHED, "Successfully schedule realm %v", realm)
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
	db.DPrintf(db.LCSCHED, "runProc kernelID %v p %v", kernelID, p)
	if err := lcs.scheddclnt.ForceRun(kernelID, false, p); err != nil {
		db.DPrintf(db.ALWAYS, "Schedd.Run %v err %v", kernelID, err)
		// Re-enqueue the proc
		lcs.addProc(p, ch)
		return
	}
	// Notify the spawner that a schedd has been chosen.
	ch <- kernelID
	// Wait for the proc to exit.
	lcs.waitProcExit(kernelID, p, r)
}

func (lcs *LCSched) waitProcExit(kernelID string, p *proc.Proc, r *Resources) {
	// RPC the schedd this proc was spawned on to wait for the proc to exit.
	db.DPrintf(db.LCSCHED, "WaitExit %v RPC", p.GetPid())
	if _, err := lcs.scheddclnt.Wait(scheddclnt.EXIT, kernelID, proc.NewProcSeqno(sp.NOT_SET, kernelID, 0, 0), p.GetPid()); err != nil {
		db.DPrintf(db.ALWAYS, "Error Schedd WaitExit: %v", err)
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
func (lcs *LCSched) addRealmQueueL(realm sp.Trealm) *schedqueue.Queue[string, chan string] {
	q := schedqueue.NewQueue[string, chan string]()
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
	sc.GetNetProxyClnt().AllowConnectionsFromAllRealms()
	lcs := NewLCSched(sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.LCSCHED, sc.ProcEnv().GetPID().String()), sc, lcs)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}

	// export queued procs through procfs. XXX maybe
	// subdirectory per realm?
	dir := procfs.NewProcDir(&QDir{lcs})
	if err := ssrv.MkNod(sp.QUEUE, dir); err != nil {
		db.DFatalf("Error mknod %v: %v", sp.QUEUE, err)
	}

	// Perf monitoring
	p, err := perf.NewPerf(sc.ProcEnv(), perf.LCSCHED)

	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}

	defer p.Done()
	go lcs.schedule()

	ssrv.RunServer()
}
