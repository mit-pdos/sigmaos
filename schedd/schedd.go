package schedd

import (
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/leasemgrsrv"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procmgr"
	"sigmaos/protdevsrv"
	"sigmaos/schedd/proto"
	"sigmaos/scheddclnt"
	sp "sigmaos/sigmap"
)

type Schedd struct {
	mu         sync.Mutex
	cond       *sync.Cond
	pmgr       *procmgr.ProcMgr
	scheddclnt *scheddclnt.ScheddClnt
	mcpufree   proc.Tmcpu
	memfree    proc.Tmem
	mfs        *memfssrv.MemFs
	qs         map[sp.Trealm]*Queue
	kernelId   string
	realms     []sp.Trealm
}

func MakeSchedd(mfs *memfssrv.MemFs, kernelId string) *Schedd {
	sd := &Schedd{
		mfs:      mfs,
		pmgr:     procmgr.MakeProcMgr(mfs, kernelId),
		qs:       make(map[sp.Trealm]*Queue),
		realms:   make([]sp.Trealm, 0),
		mcpufree: proc.Tmcpu(1000 * linuxsched.NCores),
		memfree:  mem.GetTotalMem(),
		kernelId: kernelId,
	}
	sd.cond = sync.NewCond(&sd.mu)
	sd.scheddclnt = scheddclnt.MakeScheddClnt(mfs.SigmaClnt().FsLib)
	return sd
}

func (sd *Schedd) Spawn(ctx fs.CtxI, req proto.SpawnRequest, res *proto.SpawnResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	p := proc.MakeProcFromProto(req.ProcProto)
	p.KernelId = sd.kernelId
	db.DPrintf(db.SCHEDD, "[%v] %v Spawned %v", req.Realm, sd.kernelId, p)
	if _, ok := sd.qs[sp.Trealm(req.Realm)]; !ok {
		sd.qs[sp.Trealm(req.Realm)] = makeQueue()
		sd.realms = append(sd.realms, sp.Trealm(req.Realm))
	}
	// Enqueue the proc according to its realm
	sd.qs[sp.Trealm(req.Realm)].Enqueue(p)
	s := time.Now()
	sd.pmgr.Spawn(p)
	db.DPrintf(db.SPAWN_LAT, "[%v] E2E Procmgr Spawn %v", p.GetPid(), time.Since(s))
	// Signal that a new proc may be runnable.
	sd.cond.Signal()
	return nil
}

// Steal a proc from this schedd.
func (sd *Schedd) StealProc(ctx fs.CtxI, req proto.StealProcRequest, res *proto.StealProcResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	_, res.OK = sd.qs[sp.Trealm(req.Realm)].Steal(proc.Tpid(req.PidStr))

	return nil
}

// Steal a proc from this schedd.
func (sd *Schedd) GetCPUShares(ctx fs.CtxI, req proto.GetCPUSharesRequest, res *proto.GetCPUSharesResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sm := sd.pmgr.GetCPUShares()
	smap := make(map[string]int64, len(sm))
	for r, s := range sm {
		smap[r.String()] = int64(s)
	}
	res.Shares = smap
	return nil
}

// Steal a proc from this schedd.
func (sd *Schedd) GetCPUUtil(ctx fs.CtxI, req proto.GetCPUUtilRequest, res *proto.GetCPUUtilResponse) error {
	res.Util = sd.pmgr.GetCPUUtil(sp.Trealm(req.RealmStr))
	return nil
}

func (sd *Schedd) procDone(p *proc.Proc) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	db.DPrintf(db.SCHEDD, "Proc done %v", p)
	sd.freeResourcesL(p)
	// Signal that a new proc may be runnable.
	sd.cond.Signal()
	return nil
}

// Run a proc via the local procd.
func (sd *Schedd) runProc(p *proc.Proc) {
	sd.allocResourcesL(p)
	go func() {
		sd.pmgr.RunProc(p)
		sd.procDone(p)
	}()
}

func (sd *Schedd) schedule() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Priority order in which procs are claimed
	priority := []proc.Ttype{proc.T_LC, proc.T_BE}
	for {
		var ok bool
		// Iterate through the realms round-robin.

		for _, ptype := range priority {
			for r, q := range sd.qs {
				// Try to schedule a proc from realm r.
				ok = ok || sd.tryScheduleRealmL(r, q, ptype)
			}
		}
		// If unable to schedule a proc from any realm, wait.
		if !ok {
			db.DPrintf(db.SCHEDD, "No procs runnable mcpu:%v mem:%v qs:%v", sd.mcpufree, sd.memfree, sd.qs)
			sd.cond.Wait()
		}
	}
}

// Try to schedule a proc from realm r's queue q. Returns true if a proc was
// successfully scheduled.
func (sd *Schedd) tryScheduleRealmL(r sp.Trealm, q *Queue, ptype proc.Ttype) bool {
	for {
		// Try to dequeue a proc, whether it be from a local queue or potentially
		// stolen from a remote queue.
		if p, stolen, ok := q.Dequeue(ptype, sd.mcpufree, sd.memfree); ok {
			// If the proc was stolen...
			if stolen {
				// Try to claim the proc.
				if ok := sd.tryStealProc(r, p); ok {
					// Proc was claimed successfully.
					db.DPrintf(db.SCHEDD, "[%v] stole proc %v", r, p)
				} else {
					// Couldn't claim the proc. Try and steal another.
					continue
				}
			}
			// Claimed a proc, so schedule it.
			db.DPrintf(db.SCHEDD, "[%v] run proc %v", r, p)
			db.DPrintf(db.SPAWN_LAT, "[%v] Queueing latency %v", p.GetPid(), time.Since(p.GetSpawnTime()))
			sd.runProc(p)
			return true
		} else {
			return false
		}
	}
}

func RunSchedd(kernelId string) error {
	mfs, err := memfssrv.MakeMemFs(path.Join(sp.SCHEDD, kernelId), sp.SCHEDDREL)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	lsrv := leasemgrsrv.NewLeaseSrv(mfs.GetEphemeralMap())
	_, err = leasemgrsrv.NewLeaseMgrSrv(sp.S3, mfs.SessSrv, lsrv)
	setupMemFsSrv(mfs)
	sd := MakeSchedd(mfs, kernelId)
	setupFs(mfs, sd)
	// Perf monitoring
	p, err := perf.MakePerf(perf.SCHEDD)
	if err != nil {
		db.DFatalf("Error MakePerf: %v", err)
	}
	defer p.Done()
	pds, err := protdevsrv.MakeProtDevSrvMemFs(mfs, "", sd)
	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}
	go sd.schedule()
	go sd.monitorWSQueue(proc.T_LC)
	go sd.monitorWSQueue(proc.T_BE)
	go sd.offerStealableProcs()
	pds.RunServer()
	return nil
}
