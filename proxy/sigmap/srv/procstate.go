package srv

import (
	"fmt"
	"sync"
	"time"

	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/malloc"
	"sigmaos/proc"
	wasmrt "sigmaos/proxy/wasm/rpc/wasmer"
	rpcchan "sigmaos/rpc/clnt/channel"
	sessp "sigmaos/session/proto"
	"sigmaos/shmem"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fidclnt"
	"sigmaos/sigmaclnt/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const (
	SHMEM_SIZE = 40 * sp.MBYTE
)

// Manages sigmaclnts on behalf of procs
type ProcStateMgr struct {
	mu   sync.Mutex
	spps *SPProxySrv
	ps   map[sp.Tpid]*procState // TODO: use syncmap?
}

func NewProcStateMgr(spps *SPProxySrv) *ProcStateMgr {
	return &ProcStateMgr{
		ps:   make(map[sp.Tpid]*procState),
		spps: spps,
	}
}

func (psm *ProcStateMgr) AllocProcState(pe *proc.ProcEnv, p *proc.Proc) *procState {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	// If already exists or already being created, bail out
	if ps, ok := psm.ps[pe.GetPID()]; ok {
		db.DPrintf(db.SPPROXYSRV, "AllocProcState already exists %v", pe.GetPID())
		return ps
	}

	// Otherwise, start to create the proc's state
	ps := newProcState(psm.spps, pe, p)

	// Test program may create many sigmaclnts with the same PID, so don't cache
	// them to avoid errors
	cacheState := pe.GetProgram() != "test"

	db.DPrintf(db.SPPROXYSRV, "AllocProcState newState %v", pe.GetPID())
	if cacheState {
		psm.ps[pe.GetPID()] = ps
	}
	return ps
}

func (psm *ProcStateMgr) DelProcState(p *proc.Proc) {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	ps, ok := psm.ps[p.GetPid()]
	if ok {
		if err := ps.Destroy(); err != nil {
			db.DFatalf("Err destroy proc state: %v", err)
		}
	}
	delete(psm.ps, p.GetPid())
}

func (psm *ProcStateMgr) WaitBootScriptCompletion(pid sp.Tpid) error {
	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to wait for bootscript completion for unknown proc: %v", pid)
		return fmt.Errorf("Try to wait for bootscript completion for unknown proc: %v", pid)
	}
	return ps.WaitBootScriptCompletion()
}

func (psm *ProcStateMgr) getProcState(pid sp.Tpid) (*procState, bool) {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	ps, ok := psm.ps[pid]
	return ps, ok
}

// Expects ps to be allocated already
func (psm *ProcStateMgr) GetRegisteredEPs(pid sp.Tpid) ([]*registeredEP, error) {
	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to get registered EPs for unknown proc: %v", pid)
		return nil, fmt.Errorf("Try to get registered EPs for unknown proc: %v", pid)
	}
	return ps.GetRegisteredEPs(), nil
}

// Expects ps to be allocated already
func (psm *ProcStateMgr) AddRegisteredEP(pid sp.Tpid, svcName string, instanceID string, ep *sp.Tendpoint) error {
	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to add registered EP for unknown proc: %v", pid)
		return fmt.Errorf("Try to add registered EP for unknown proc: %v", pid)
	}
	ps.AddRegisteredEP(svcName, instanceID, ep)
	return nil
}

// Expects ps to be allocated already
func (psm *ProcStateMgr) GetSigmaClnt(pid sp.Tpid) (*sigmaclnt.SigmaClnt, *epcacheclnt.EndpointCacheClnt, error) {
	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to get sigmaclnt for unknown proc: %v", pid)
		return nil, nil, fmt.Errorf("Try to get sigmaclnt for unknown proc: %v", pid)
	}
	return ps.GetSigmaClnt()
}

func (psm *ProcStateMgr) GetShmemAllocator(pid sp.Tpid) (malloc.Allocator, error) {
	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to get shmalloc for unknown proc: %v", pid)
		return nil, fmt.Errorf("Try to get shmalloc for unknown proc: %v", pid)
	}
	return ps.GetShmemAllocator(), nil
}

func (psm *ProcStateMgr) InsertReply(p *proc.Proc, rpcIdx uint64, iov *sessp.IoVec, err error, start time.Time) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.InsertReply(%v) lat=%v", p.GetPid(), rpcIdx, time.Since(start))
	perf.LogSpawnLatency("DelegatedRPC(%v)", p.GetPid(), p.GetSpawnTime(), start, rpcIdx)
	ps, ok := psm.getProcState(p.GetPid())
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to insert delegated RPC reply for unknown proc: %v", p.GetPid())
		return
	}
	ps.rpcReps.InsertReply(rpcIdx, iov, err)
}

func (psm *ProcStateMgr) GetReply(pid sp.Tpid, rpcIdx uint64) (*sessp.IoVec, error) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetReply(%v)", pid, rpcIdx)
	defer db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetReply(%v) done", pid, rpcIdx)

	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to get delegated RPC reply for unknown proc: %v", pid)
		return nil, fmt.Errorf("Try to get delegated RPC reply for unknown proc: %v", pid)
	}
	return ps.rpcReps.GetReply(rpcIdx)
}

func (psm *ProcStateMgr) GetShmemBuf(pid sp.Tpid) ([]byte, error) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetShmemBuf", pid)
	defer db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetShmemBuf done", pid)

	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to get shmem buf for unknown proc: %v", pid)
		return nil, fmt.Errorf("Try to get shmem buf for unknown proc: %v", pid)
	}
	return ps.shm.GetBuf(), nil
}

func (psm *ProcStateMgr) GetRPCChannel(sc *sigmaclnt.SigmaClnt, pid sp.Tpid, rpcIdx uint64, pn string) (rpcchan.RPCChannel, error) {
	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to get delegated RPC reply for unknown proc: %v", pid)
		return nil, fmt.Errorf("Can't find proc stae: %v", pid)
	}
	return ps.rpcReps.GetRPCChannel(sc, rpcIdx, pn)
}

type registeredEP struct {
	svcName      string
	instanceName string
	ep           *sp.Tendpoint
}

func newRegisteredEP(svcName string, instanceName string, ep *sp.Tendpoint) *registeredEP {
	return &registeredEP{
		svcName:      svcName,
		instanceName: instanceName,
		ep:           ep,
	}
}

func (rep *registeredEP) String() string {
	return fmt.Sprintf("&{ svcName:%v instanceName:%v ep:%v }", rep.svcName, rep.instanceName, rep.ep)
}

type procState struct {
	mu                       sync.Mutex
	cond                     *sync.Cond
	bsCond                   *sync.Cond
	spps                     *SPProxySrv
	sigmaClntCreationStarted bool
	done                     bool // done creating the proc state?
	bootScriptCompleted      bool
	pe                       *proc.ProcEnv
	p                        *proc.Proc
	rpcReps                  *RPCState
	eps                      []*registeredEP
	wrt                      *wasmrt.WasmerRuntime
	sc                       *sigmaclnt.SigmaClnt
	epcc                     *epcacheclnt.EndpointCacheClnt
	shm                      *shmem.Segment
	shmAlloc                 malloc.Allocator
	err                      error // Creation result
	bsErr                    error // BootScript result
}

func newProcState(spps *SPProxySrv, pe *proc.ProcEnv, p *proc.Proc) *procState {
	ps := &procState{
		pe:      pe,
		p:       p,
		eps:     make([]*registeredEP, 0, 1),
		rpcReps: NewRPCState(),
		done:    false,
		spps:    spps,
	}
	ps.cond = sync.NewCond(&ps.mu)
	ps.bsCond = sync.NewCond(&ps.mu)
	if pe.GetUseShmem() {
		var err error
		start := time.Now()
		ps.shm, err = shmem.NewSegment(pe.GetPID().String(), SHMEM_SIZE)
		if err != nil {
			db.DFatalf("Err shmem NewSegment: %v", err)
		}
		ps.shmAlloc = shmem.NewAllocator(ps.shm)
		perf.LogSpawnLatency("SPProxySrv.shmem.NewSegment", ps.pe.GetPID(), ps.pe.GetSpawnTime(), start)
	}
	if ps.p.GetRunBootScript() {
		ps.sigmaClntCreationStarted = true
		go ps.createSigmaClnt(spps)
	} else {
		ps.bootScriptCompleted = true
	}
	return ps
}

func (ps *procState) AddRegisteredEP(svcName string, instanceName string, ep *sp.Tendpoint) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.eps = append(ps.eps, newRegisteredEP(svcName, instanceName, ep))
}

func (ps *procState) GetRegisteredEPs() []*registeredEP {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	eps := make([]*registeredEP, len(ps.eps))
	for i := range ps.eps {
		eps[i] = ps.eps[i]
	}
	return eps
}

func (ps *procState) GetShmemAllocator() malloc.Allocator {
	return ps.shmAlloc
}

func (ps *procState) GetSigmaClnt() (*sigmaclnt.SigmaClnt, *epcacheclnt.EndpointCacheClnt, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.sigmaClntCreationStarted {
		ps.sigmaClntCreationStarted = true
		// If the sigma clnt creation hasn't started yet, start it
		go ps.createSigmaClnt(ps.spps)
	}

	for !ps.done {
		ps.cond.Wait()
	}
	return ps.sc, ps.epcc, ps.err
}

func (ps *procState) setSigmaClnt(sc *sigmaclnt.SigmaClnt, epcc *epcacheclnt.EndpointCacheClnt, err error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.sc = sc
	ps.epcc = epcc
	ps.err = err

	ps.done = true

	ps.cond.Broadcast()
}

func (ps *procState) WaitBootScriptCompletion() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for !ps.bootScriptCompleted {
		ps.bsCond.Wait()
	}
	return ps.bsErr
}

func (ps *procState) bootScriptDone(err error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.bootScriptCompleted = true
	ps.bsCond.Broadcast()
	ps.bsErr = err
}

func (ps *procState) Destroy() error {
	if ps.shm != nil {
		return ps.shm.Destroy()
	}
	return nil
}

func (ps *procState) createSigmaClnt(spps *SPProxySrv) {
	db.DPrintf(db.SPPROXYSRV, "createSigmaClnt for %v withProcClnt %v", ps.pe.GetPID(), ps.pe.UseSPProxyProcClnt)
	start := time.Now()
	sc, err := sigmaclnt.NewSigmaClntFsLibFidClnt(ps.pe, fidclnt.NewFidClnt(ps.pe, dialproxyclnt.NewDialProxyClnt(ps.pe)))
	perf.LogSpawnLatency("SPProxySrv.createSigmaClnt initFsLib", ps.pe.GetPID(), ps.pe.GetSpawnTime(), start)
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "Error NewSigmaClnt proc %v", ps.pe.GetPID())
	}
	if ps.p != nil && ps.p.GetRunBootScript() {
		start := time.Now()
		// If the proc specified a boot script, create a WASM runtime and run the
		// script
		rpcAPI := NewWASMRPCProxy(spps, sc, ps.p)
		ps.wrt = wasmrt.NewWasmerRuntime(rpcAPI)
		perf.LogSpawnLatency("Create wasmRT", ps.pe.GetPID(), ps.pe.GetSpawnTime(), start)
		go func() {
			err := ps.wrt.RunModule(ps.p.GetPid(), ps.p.GetSpawnTime(), ps.p.GetBootScript(), ps.p.GetBootScriptInput())
			ps.bootScriptDone(err)
		}()
	}
	var epcc *epcacheclnt.EndpointCacheClnt
	// Initialize a procclnt too
	if err == nil {
		if ps.pe.UseSPProxyProcClnt {
			start := time.Now()
			err = sc.NewProcClnt()
			perf.LogSpawnLatency("SPProxySrv.createSigmaClnt initProcClnt", ps.pe.GetPID(), ps.pe.GetSpawnTime(), start)
			if err != nil {
				db.DPrintf(db.SPPROXYSRV_ERR, "%v: Failed to create procclnt: %v", ps.pe.GetPID(), err)
			} else {
				// Initialize the procclnt's connection to msched, which will be needed to
				// call, e.g., Started. This can be done asynchronously.
				go func() {
					err = sc.ProcAPI.(*procclnt.ProcClnt).InitMSchedClnt()
					perf.LogSpawnLatency("SPProxySrv.createSigmaClnt initMSchedClnt", ps.pe.GetPID(), ps.pe.GetSpawnTime(), start)
					if err != nil {
						db.DPrintf(db.SPPROXYSRV_ERR, "%v: Failed to initialize msched clnt: %v", ps.pe.GetPID(), err)
					}
				}()
			}
		}
		// If running with EPCache, pre-mount the epcache srv
		if epcsrvEP, ok := ps.pe.GetCachedEndpoint(epcache.EPCACHE); ok {
			if err := epcacheclnt.MountEPCacheSrv(sc.FsLib, epcsrvEP); err != nil {
				db.DPrintf(db.SPPROXYSRV_ERR, "%v: failed to mount EPCacheSrv EP: %v", ps.pe.GetPID(), err)
			}
			epcc, err = epcacheclnt.NewEndpointCacheClnt(sc.FsLib)
			if err != nil {
				db.DPrintf(db.SPPROXYSRV_ERR, "%v: Err NewEPCacheClnt: %v", ps.pe.GetPID(), err)
			}
		}
	}
	ps.setSigmaClnt(sc, epcc, err)
}
