package srv

import (
	"sync"
	"time"

	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fidclnt"
	"sigmaos/sigmaclnt/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

// Manages sigmaclnts on behalf of procs
type SigmaClntMgr struct {
	mu   sync.Mutex
	spps *SPProxySrv
	ps   map[sp.Tpid]*procState // TODO: use syncmap?
}

func NewSigmaClntMgr(spps *SPProxySrv) *SigmaClntMgr {
	return &SigmaClntMgr{
		ps:   make(map[sp.Tpid]*procState),
		spps: spps,
	}
}

func (scm *SigmaClntMgr) AllocProcState(pe *proc.ProcEnv, p *proc.Proc) *procState {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	// If already exists or already being created, bail out
	if ps, ok := scm.ps[pe.GetPID()]; ok {
		db.DPrintf(db.SPPROXYSRV, "AllocProcState already exists %v", pe.GetPID())
		return ps
	}

	// Otherwise, start to create the proc's state
	ps := newProcState(scm.spps, pe, p)

	// Test program may create many sigmaclnts with the same PID, so don't cache
	// them to avoid errors
	cacheState := pe.GetProgram() != "test"

	db.DPrintf(db.SPPROXYSRV, "AllocProcState newState %v", pe.GetPID())
	if cacheState {
		scm.ps[pe.GetPID()] = ps
	}
	return ps
}

func (scm *SigmaClntMgr) DelProcState(p *proc.Proc) {
	delete(scm.ps, p.GetPid())
}

// Expects ps to be allocated already
func (scm *SigmaClntMgr) GetSigmaClnt(pid sp.Tpid) (*sigmaclnt.SigmaClnt, *epcacheclnt.EndpointCacheClnt, error) {
	scm.mu.Lock()
	ps := scm.ps[pid]
	scm.mu.Unlock()

	return ps.GetSigmaClnt()
}

type procState struct {
	mu   sync.Mutex
	cond *sync.Cond
	done bool // done creating the proc state?
	pe   *proc.ProcEnv
	p    *proc.Proc
	sc   *sigmaclnt.SigmaClnt
	epcc *epcacheclnt.EndpointCacheClnt
	err  error // Creation result
}

func newProcState(spps *SPProxySrv, pe *proc.ProcEnv, p *proc.Proc) *procState {
	ps := &procState{
		pe:   pe,
		p:    p,
		done: false,
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
		// Set up delegated RPC reply table for the proc
		spps.repTab.NewProc(ps.p)
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
