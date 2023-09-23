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
	mfs       *memfssrv.MemFs
	kernelId  string
	rootsc    *sigmaclnt.SigmaClnt
	updm      *uprocclnt.UprocdMgr
	sclnts    map[sp.Trealm]*sigmaclnt.SigmaClnt
	cachedirs map[sp.Trealm]bool
	running   map[sp.Tpid]*proc.Proc
	pstate    *ProcState
}

// Manages the state and lifecycle of a proc.
func NewProcMgr(mfs *memfssrv.MemFs, kernelId string) *ProcMgr {
	mgr := &ProcMgr{
		mfs:       mfs,
		kernelId:  kernelId,
		rootsc:    mfs.SigmaClnt(),
		updm:      uprocclnt.NewUprocdMgr(mfs.SigmaClnt().FsLib, kernelId),
		sclnts:    make(map[sp.Trealm]*sigmaclnt.SigmaClnt),
		cachedirs: make(map[sp.Trealm]bool),
		running:   make(map[sp.Tpid]*proc.Proc),
		pstate:    NewProcState(),
	}
	return mgr
}

// Proc has been spawned.
func (mgr *ProcMgr) Spawn(p *proc.Proc) {
	db.DPrintf(db.SPAWN_LAT, "[%v] Proc spawn time %v", p.GetPid(), time.Since(p.GetSpawnTime()))
	mgr.pstate.spawn(p)
}

func (mgr *ProcMgr) RunProc(p *proc.Proc) {
	s := time.Now()
	// Set the proc's kernel ID, now that a kernel has been selected to run the
	// proc.
	p.SetKernelID(mgr.kernelId, true)
	mgr.setupProcState(p)
	db.DPrintf(db.SPAWN_LAT, "[%v] Proc state setup %v", p.GetPid(), time.Since(s))
	s = time.Now()
	mgr.downloadProc(p)
	db.DPrintf(db.SPAWN_LAT, "[%v] Binary download time %v", p.GetPid(), time.Since(s))
	mgr.runProc(p)
	// Ensure that the proc is marked as exited after it has run. The
	// proc may not have done this if, for example, it crashed.
	mgr.pstate.exited(p.GetPid())
	mgr.teardownProcState(p)
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

func (mgr *ProcMgr) Exited(pid sp.Tpid) {
	mgr.pstate.exited(pid)
}

func (mgr *ProcMgr) WaitExit(pid sp.Tpid) {
	mgr.pstate.waitExit(pid)
}

func (mgr *ProcMgr) GetCPUShares() map[sp.Trealm]uprocclnt.Tshare {
	return mgr.updm.GetCPUShares()
}

func (mgr *ProcMgr) GetCPUUtil(realm sp.Trealm) float64 {
	return mgr.updm.GetCPUUtil(realm)
}

func (mgr *ProcMgr) getSigmaClnt(realm sp.Trealm) *sigmaclnt.SigmaClnt {
	mgr.Lock()
	defer mgr.Unlock()

	var clnt *sigmaclnt.SigmaClnt
	var ok bool
	if clnt, ok = mgr.sclnts[realm]; !ok {
		// No need to make a new client for the root realm.
		if realm == sp.ROOTREALM {
			clnt = mgr.rootsc
		} else {
			var err error
			pcfg := proc.NewDifferentRealmProcEnv(mgr.rootsc.ProcEnv(), realm)
			if clnt, err = sigmaclnt.NewSigmaClnt(pcfg); err != nil {
				db.DFatalf("Err NewSigmaClntRealm: %v", err)
			}
			// Mount KPIDS.
			procclnt.MountPids(clnt.FsLib)
		}
		mgr.sclnts[realm] = clnt
	}
	return clnt
}
