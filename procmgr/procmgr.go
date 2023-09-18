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
	pcache    *ProcCache
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
		pcache:    NewProcCache(PROC_CACHE_SZ),
	}
	mgr.newws()
	return mgr
}

// Create ws queue if it doesn't exist
func (mgr *ProcMgr) newws() {
	mgr.rootsc.NewDir(sp.WS, 0777)
	for _, n := range []string{sp.WS_RUNQ_LC, sp.WS_RUNQ_BE} {
		mgr.rootsc.NewDir(n, 0777)
	}
}

// Proc has been spawned.
func (mgr *ProcMgr) Spawn(p *proc.Proc) {
	// XXX will probably kill this eventually
	mgr.postProcInQueue(p)

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
	// Ensure that the proc is marked as started & exited after it has run. The
	// proc may not have done this if, for example, it crashed.
	mgr.pstate.started(p.GetPid())
	mgr.pstate.exited(p.GetPid())
	mgr.teardownProcState(p)
}

func (mgr *ProcMgr) Started(pid sp.Tpid) {
	mgr.pstate.started(pid)
}

func (mgr *ProcMgr) WaitStart(pid sp.Tpid) {
	mgr.pstate.waitStart(pid)
}

func (mgr *ProcMgr) Exited(pid sp.Tpid) {
	mgr.pstate.exited(pid)
}

func (mgr *ProcMgr) WaitExit(pid sp.Tpid) {
	mgr.pstate.waitExit(pid)
}

// Try to steal a proc from another schedd. Must be callled after RPCing the
// victim schedd.
func (mgr *ProcMgr) TryStealProc(p *proc.Proc) {
	// Remove the proc from the ws queue. This can only be done *after* RPCing
	// schedd. Otherwise, if this proc crashes after removing the stealable proc
	// but before claiming it from the victim schedd, the proc will not be added
	// back to the WS queue, and other schedds will not have the opportunity to
	// steal it.
	//
	// It is safe, however, to remove the proc regardless of whether or not the
	// steal is actually successful. If the steal is unsuccessful, that means
	// another schedd was granted the proc by the victim, and will remove it
	// anyway. Eagerly removing it here stops additional schedds from trying to
	// steal it in the intervening time.
	mgr.removeWSLink(p)
}

func (mgr *ProcMgr) OfferStealableProc(p *proc.Proc) {
	mgr.createWSLink(p)
}

// Get the contents of the WS Queue for procs of type ptype.
func (mgr *ProcMgr) GetWSQueue(ptype proc.Ttype) (map[sp.Trealm][]*proc.Proc, bool) {
	return mgr.getWSQueue(getWSQueuePath(ptype))
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
			var err error
			if clnt, err = sigmaclnt.NewSigmaLeaseClnt(mgr.rootsc.FsLib); err != nil {
				db.DFatalf("Err NewSigmaLeaseClnt: %v", err)
			}
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
