package schedd

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procmgr"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
)

type Schedd struct {
	mu        sync.Mutex
	cond      *sync.Cond
	pmgr      *procmgr.ProcMgr
	schedds   map[string]*protdevclnt.ProtDevClnt
	ranBE     bool
	coresfree proc.Tcore
	memfree   proc.Tmem
	mfs       *memfssrv.MemFs
	qs        map[sp.Trealm]*Queue
	realms    []sp.Trealm
}

func MakeSchedd(mfs *memfssrv.MemFs) *Schedd {
	sd := &Schedd{
		mfs:       mfs,
		pmgr:      procmgr.MakeProcMgr(mfs),
		qs:        make(map[sp.Trealm]*Queue),
		realms:    make([]sp.Trealm, 0),
		schedds:   make(map[string]*protdevclnt.ProtDevClnt),
		ranBE:     false,
		coresfree: proc.Tcore(linuxsched.NCores),
		memfree:   mem.GetTotalMem(),
	}
	sd.cond = sync.NewCond(&sd.mu)
	return sd
}

func (sd *Schedd) Spawn(req proto.SpawnRequest, res *proto.SpawnResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	p := proc.MakeProcFromProto(req.ProcProto)
	p.ScheddIp = sd.mfs.MyAddr()
	db.DPrintf(db.SCHEDD, "[%v] Spawned %v", req.Realm, p)
	if _, ok := sd.qs[sp.Trealm(req.Realm)]; !ok {
		sd.qs[sp.Trealm(req.Realm)] = makeQueue()
		sd.realms = append(sd.realms, sp.Trealm(req.Realm))
	}
	// Enqueue the proc according to its realm
	sd.qs[sp.Trealm(req.Realm)].Enqueue(p)
	sd.pmgr.Spawn(p)
	// Signal that a new proc may be runnable.
	sd.cond.Signal()
	return nil
}

// Steal a proc from this schedd.
func (sd *Schedd) StealProc(req proto.StealProcRequest, res *proto.StealProcResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	_, res.OK = sd.qs[sp.Trealm(req.Realm)].Steal(proc.Tpid(req.PidStr))

	return nil
}

// Steal a proc from this schedd.
func (sd *Schedd) GetCPUShares(req proto.GetCPUSharesRequest, res *proto.GetCPUSharesResponse) error {
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
func (sd *Schedd) GetCPUUtil(req proto.GetCPUUtilRequest, res *proto.GetCPUUtilResponse) error {
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

// TODO: Proper fair-share scheduling policy, and more fine-grained locking.
func (sd *Schedd) schedule() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	for {
		var ok bool
		// Iterate through the realms round-robin.
		for r, q := range sd.qs {
			// Try to schedule a proc from realm r.
			ok = ok || sd.tryScheduleRealmL(r, q)
		}
		// If unable to schedule a proc from any realm, wait.
		if !ok {
			db.DPrintf(db.SCHEDD, "No procs runnable cores:%v mem:%v qs:%v", sd.coresfree, sd.memfree, sd.qs)
			sd.cond.Wait()
		}
	}
}

// Try to schedule a proc from realm r's queue q. Returns true if a proc was
// successfully scheduled.
func (sd *Schedd) tryScheduleRealmL(r sp.Trealm, q *Queue) bool {
	for {
		// Try to dequeue a proc, whether it be from a local queue or potentially
		// stolen from a remote queue.
		if p, stolen, ok := q.Dequeue(sd.ranBE, sd.coresfree, sd.memfree); ok {
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
			sd.runProc(p)
			return true
		} else {
			return false
		}
	}
}

func RunSchedd() error {
	mfs, _, err := memfssrv.MakeMemFs(sp.SCHEDD, sp.SCHEDDREL)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	setupMemFsSrv(mfs)
	sd := MakeSchedd(mfs)
	setupFs(mfs, sd)
	// Perf monitoring
	p, err := perf.MakePerf(perf.SCHEDD)
	if err != nil {
		db.DFatalf("Error MakePerf: %v", err)
	}
	defer p.Done()
	pds, err := protdevsrv.MakeProtDevSrvMemFs(mfs, sd)
	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}
	go sd.schedule()
	go sd.monitorWSQueue(proc.T_LC)
	go sd.monitorWSQueue(proc.T_BE)
	go sd.offerStealableProcs()
	return pds.RunServer()
}
