package procmgr

import (
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/uprocclnt"
)

const (
	PROC_CACHE_SZ = 500
)

type ProcMgr struct {
	sync.Mutex
	mfs            *memfssrv.MemFs
	kernelId       string
	rootsc         *sigmaclnt.SigmaClntKernel
	updm           *uprocclnt.UprocdMgr
	sclnts         map[sp.Trealm]*sigmaclnt.SigmaClntKernel
	cachedProcBins map[sp.Trealm]map[string]bool
	pstate         *ProcState
}

// Manages the state and lifecycle of a proc.
func NewProcMgr(sc *sigmaclnt.SigmaClnt, kernelId string) *ProcMgr {
	mgr := &ProcMgr{
		kernelId:       kernelId,
		rootsc:         sigmaclnt.NewSigmaClntKernel(sc),
		updm:           uprocclnt.NewUprocdMgr(sc.FsLib, kernelId),
		sclnts:         make(map[sp.Trealm]*sigmaclnt.SigmaClntKernel),
		cachedProcBins: make(map[sp.Trealm]map[string]bool),
		pstate:         NewProcState(),
	}
	return mgr
}

// Proc has been spawned.
func (mgr *ProcMgr) Spawn(p *proc.Proc) {
	db.DPrintf(db.SPAWN_LAT, "[%v] MSched proc time since spawn %v", p.GetPid(), time.Since(p.GetSpawnTime()))
	mgr.pstate.spawn(p)
}

func (mgr *ProcMgr) SetMemFs(mfs *memfssrv.MemFs) {
	mgr.mfs = mfs
}

func (mgr *ProcMgr) RunProc(p *proc.Proc) {
	// Set the proc's kernel ID, now that a kernel has been selected to run the
	// proc.
	p.SetKernelID(mgr.kernelId, true)
	// Set the schedd mount for the proc, so it can mount this schedd in one RPC
	// (without walking down to it).
	p.SetMSchedEndpoint(mgr.mfs.GetSigmaPSrvEndpoint())
	mgr.setupProcState(p)
	err := mgr.runProc(p)
	if err != nil {
		mgr.procCrashed(p, err)
	}
}

func (mgr *ProcMgr) Started(pid sp.Tpid) {
	mgr.pstate.started(pid)
}

func (mgr *ProcMgr) WaitStart(pid sp.Tpid) {
	mgr.pstate.waitStart(pid)
}

func (mgr *ProcMgr) Evict(pid sp.Tpid) {
	mgr.pstate.evict(pid)
}

func (mgr *ProcMgr) WaitEvict(pid sp.Tpid) {
	mgr.pstate.waitEvict(pid)
}

func (mgr *ProcMgr) Exited(pid sp.Tpid, status []byte) {
	mgr.pstate.exited(pid, status)
}

func (mgr *ProcMgr) WaitExit(pid sp.Tpid) []byte {
	return mgr.pstate.waitExit(pid)
}

func (mgr *ProcMgr) GetCPUShares() map[sp.Trealm]uprocclnt.Tshare {
	return mgr.updm.GetCPUShares()
}

func (mgr *ProcMgr) GetCPUUtil(realm sp.Trealm) float64 {
	return mgr.updm.GetCPUUtil(realm)
}

func (mgr *ProcMgr) GetRunningProcs() []*proc.Proc {
	return mgr.pstate.GetProcs()
}

func (mgr *ProcMgr) WarmUprocd(pid sp.Tpid, realm sp.Trealm, prog string, path []string, ptype proc.Ttype) error {
	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.REALM_GROW_LAT, "[%v.%v] WarmUprocd latency: %v", realm, prog, time.Since(start))
	}(start)
	// Warm up sigmaclnt
	mgr.getSigmaClnt(realm)
	if err := mgr.updm.WarmStartUprocd(realm, ptype); err != nil {
		db.DPrintf(db.ERROR, "WarmStartUprocd %v err %v", realm, err)
		return err
	}
	if uprocErr, childErr := mgr.updm.WarmProc(pid, realm, prog, path, ptype); childErr != nil {
		return childErr
	} else if uprocErr != nil {
		// Unexpected error with uproc server.
		db.DPrintf(db.PROCMGR, "WarmUproc err %v", uprocErr)
		return uprocErr
	}
	return nil
}

// Set up state to notify parent that a proc crashed.
func (mgr *ProcMgr) procCrashed(p *proc.Proc, err error) {
	// Mark the proc as exited due to a crash, and record the error exit status.
	mgr.pstate.exited(p.GetPid(), proc.NewStatusErr(err.Error(), nil).Marshal())
	db.DPrintf(db.PROCMGR_ERR, "Proc %v finished with error: %v", p, err)
	mgr.getSigmaClnt(p.GetRealm()).ExitedCrashed(p.GetPid(), p.GetProcDir(), p.GetParentDir(), proc.NewStatusErr(err.Error(), nil), p.GetHow())
}

func (mgr *ProcMgr) getSigmaClnt(realm sp.Trealm) *sigmaclnt.SigmaClntKernel {
	mgr.Lock()
	defer mgr.Unlock()

	return mgr.getSigmaClntL(realm)
}

func (mgr *ProcMgr) getSigmaClntL(realm sp.Trealm) *sigmaclnt.SigmaClntKernel {
	var clnt *sigmaclnt.SigmaClntKernel
	var ok bool
	if clnt, ok = mgr.sclnts[realm]; !ok {
		// No need to make a new client for the root realm.
		if realm == sp.ROOTREALM {
			clnt = mgr.rootsc
		} else {
			pe := proc.NewDifferentRealmProcEnv(mgr.rootsc.ProcEnv(), realm)
			if sc, err := sigmaclnt.NewSigmaClnt(pe); err != nil {
				db.DFatalf("Err NewSigmaClntRealm: %v", err)
			} else {
				// Endpoint KPIDS.
				clnt = sigmaclnt.NewSigmaClntKernel(sc)
				if err := procclnt.MountPids(clnt.FsLib); err != nil {
					db.DFatalf("Error MountPids: %v", err)
				}
			}
		}
		mgr.sclnts[realm] = clnt
	}
	return clnt
}
