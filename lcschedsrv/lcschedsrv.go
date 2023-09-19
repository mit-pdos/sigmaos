package lcschedsrv

import (
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	proto "sigmaos/lcschedsrv/proto"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	pqproto "sigmaos/procqsrv/proto"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type LCSchedSrv struct {
	sync.Mutex
	cond    *sync.Cond
	mfs     *memfssrv.MemFs
	qs      map[sp.Trealm]*Queue
	schedds map[string]*Resources
}

func NewLCSchedSrv(mfs *memfssrv.MemFs) *LCSchedSrv {
	lcs := &LCSchedSrv{
		mfs: mfs,
		qs:  make(map[sp.Trealm]*Queue),
	}
	lcs.cond = sync.NewCond(&lcs.Mutex)
	return lcs
}

func (lcs *LCSchedSrv) Enqueue(ctx fs.CtxI, req pqproto.EnqueueRequest, res *pqproto.EnqueueResponse) error {
	p := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.LCSCHED, "[%v] Enqueued %v", p.GetRealm(), p)

	ch := lcs.addProc(p)
	res.KernelID = <-ch
	return nil
}

func (lcs *LCSchedSrv) RegisterSchedd(ctx fs.CtxI, req proto.RegisterScheddRequest, res *proto.RegisterScheddResponse) error {
	lcs.Lock()
	defer lcs.Unlock()

	db.DPrintf(db.LCSCHED, "Register Schedd id:%v mcpu:%v mem:%v", req.KernelID, req.McpuInt, req.MemInt)
	if _, ok := lcs.schedds[req.KernelID]; ok {
		db.DFatalf("Double-register schedd %v", req.KernelID)
	}
	lcs.schedds[req.KernelID] = newResources(req.McpuInt, req.MemInt)
	return nil
}

func (lcs *LCSchedSrv) addProc(p *proc.Proc) chan string {
	lcs.Lock()
	defer lcs.Unlock()

	q, ok := lcs.getQueue(p.GetRealm())
	if !ok {
		q = lcs.addRealmQueueL(p.GetRealm())
	}
	// Enqueue the proc according to its realm
	ch := q.Enqueue(p)
	// Signal that a new proc may be runnable.
	lcs.cond.Signal()
	return ch
}

// Caller holds lock.
func (lcs *LCSchedSrv) getQueue(realm sp.Trealm) (*Queue, bool) {
	q, ok := lcs.qs[realm]
	return q, ok
}

// Caller must hold lock.
func (lcs *LCSchedSrv) addRealmQueueL(realm sp.Trealm) *Queue {
	q := newQueue()
	lcs.qs[realm] = q
	return q
}

// Run an LCSchedSrv
func Run() {
	pcfg := proc.GetProcEnv()
	mfs, err := memfssrv.NewMemFs(path.Join(sp.LCSCHED, pcfg.GetPID().String()), pcfg)

	if err != nil {
		db.DFatalf("Error NewMemFs: %v", err)
	}

	lcs := NewLCSchedSrv(mfs)
	ssrv, err := sigmasrv.NewSigmaSrvMemFs(mfs, lcs)

	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}

	setupMemFsSrv(ssrv.MemFs)
	setupFs(ssrv.MemFs)
	// Perf monitoring
	p, err := perf.NewPerf(pcfg, perf.LCSCHED)

	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}

	defer p.Done()

	ssrv.RunServer()
}
