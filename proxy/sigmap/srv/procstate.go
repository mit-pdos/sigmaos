package srv

import (
	"fmt"
	"sync"
	"time"

	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	wasmrt "sigmaos/proxy/wasm/rpc/wasmer"
	rpcchan "sigmaos/rpc/clnt/channel"
	sessp "sigmaos/session/proto"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fidclnt"
	"sigmaos/sigmaclnt/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
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

	delete(psm.ps, p.GetPid())
}

func (psm *ProcStateMgr) getProcState(pid sp.Tpid) (*procState, bool) {
	psm.mu.Lock()
	defer psm.mu.Unlock()

	ps, ok := psm.ps[pid]
	return ps, ok
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

func (psm *ProcStateMgr) InsertReply(p *proc.Proc, rpcIdx uint64, iov sessp.IoVec, err error, start time.Time) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.InsertReply(%v) lat=%v", p.GetPid(), rpcIdx, time.Since(start))
	perf.LogSpawnLatency("DelegatedRPC(%v)", p.GetPid(), p.GetSpawnTime(), start, rpcIdx)
	ps, ok := psm.getProcState(p.GetPid())
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to insert delegated RPC reply for unknown proc: %v", p.GetPid())
		return
	}
	ps.rpcReps.InsertReply(rpcIdx, iov, err)
}

func (psm *ProcStateMgr) GetReply(pid sp.Tpid, rpcIdx uint64) (sessp.IoVec, error) {
	db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetReply(%v)", pid, rpcIdx)
	defer db.DPrintf(db.SPPROXYSRV, "[%v] DelegatedRPC.GetReply(%v) done", pid, rpcIdx)

	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to get delegated RPC reply for unknown proc: %v", pid)
		return nil, fmt.Errorf("Try to get delegated RPC reply for unknown proc: %v", pid)
	}
	return ps.rpcReps.GetReply(rpcIdx)
}

func (psm *ProcStateMgr) GetRPCChannel(pid sp.Tpid, pn string) (rpcchan.RPCChannel, bool) {
	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to get delegated RPC reply for unknown proc: %v", pid)
		return nil, false
	}
	return ps.rpcReps.GetRPCChannel(pn)
}

func (psm *ProcStateMgr) PutRPCChannel(pid sp.Tpid, pn string, ch rpcchan.RPCChannel) {
	ps, ok := psm.getProcState(pid)
	if !ok {
		db.DPrintf(db.SPPROXYSRV_ERR, "Try to RPC channel for unknown proc: %v", pid)
		return
	}
	ps.rpcReps.PutRPCChannel(pn, ch)
}

type procState struct {
	mu      sync.Mutex
	cond    *sync.Cond
	done    bool // done creating the proc state?
	pe      *proc.ProcEnv
	p       *proc.Proc
	rpcReps *RPCState
	wrt     *wasmrt.WasmerRuntime
	sc      *sigmaclnt.SigmaClnt
	epcc    *epcacheclnt.EndpointCacheClnt
	err     error // Creation result
}

func newProcState(spps *SPProxySrv, pe *proc.ProcEnv, p *proc.Proc) *procState {
	ps := &procState{
		pe:      pe,
		p:       p,
		rpcReps: NewRPCState(),
		done:    false,
	}
	ps.cond = sync.NewCond(&ps.mu)
	go ps.createSigmaClnt(spps)
	return ps
}

func (ps *procState) GetSigmaClnt() (*sigmaclnt.SigmaClnt, *epcacheclnt.EndpointCacheClnt, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

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

func (ps *procState) createSigmaClnt(spps *SPProxySrv) {
	db.DPrintf(db.SPPROXYSRV, "createSigmaClnt for %v withProcClnt %v", ps.pe.GetPID(), ps.pe.UseSPProxyProcClnt)
	start := time.Now()
	sc, err := sigmaclnt.NewSigmaClntFsLibFidClnt(ps.pe, fidclnt.NewFidClnt(ps.pe, dialproxyclnt.NewDialProxyClnt(ps.pe)))
	perf.LogSpawnLatency("SPProxySrv.createSigmaClnt initFsLib", ps.pe.GetPID(), ps.pe.GetSpawnTime(), start)
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "Error NewSigmaClnt proc %v", ps.pe.GetPID())
	}
	// We only need an fslib to run delegated RPCs
	if ps.p != nil {
		go spps.runDelegatedInitializationRPCs(ps.p, sc)
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
				// call, e.g., Started
				err = sc.ProcAPI.(*procclnt.ProcClnt).InitMSchedClnt()
				perf.LogSpawnLatency("SPProxySrv.createSigmaClnt initMSchedClnt", ps.pe.GetPID(), ps.pe.GetSpawnTime(), start)
				if err != nil {
					db.DPrintf(db.SPPROXYSRV_ERR, "%v: Failed to initialize msched clnt: %v", ps.pe.GetPID(), err)
				}
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
