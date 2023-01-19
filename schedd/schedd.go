package schedd

import (
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	procdproto "sigmaos/procd/proto"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
)

type Schedd struct {
	mu        sync.Mutex
	cond      *sync.Cond
	procdIp   string
	procd     *protdevclnt.ProtDevClnt
	coresfree proc.Tcore
	memfree   proc.Tmem
	mfs       *memfssrv.MemFs
	qs        map[string]*Queue
}

func MakeSchedd(mfs *memfssrv.MemFs) *Schedd {
	sd := &Schedd{
		mfs:       mfs,
		qs:        make(map[string]*Queue),
		coresfree: proc.Tcore(linuxsched.NCores),
		memfree:   mem.GetTotalMem(),
	}
	sd.cond = sync.NewCond(&sd.mu)
	return sd
}

func (sd *Schedd) RegisterProcd(req proto.RegisterRequest, res *proto.RegisterResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	if sd.procdIp != "" {
		db.DFatalf("Register procd on schedd with procd already registered")
	}
	sd.procdIp = req.ProcdIp
	var err error
	sd.procd, err = protdevclnt.MkProtDevClnt(sd.mfs.FsLib(), path.Join(sp.PROCD, sd.procdIp))
	if err != nil {
		db.DFatalf("Error make procd clnt: %v", err)
	}
	db.DPrintf(db.SCHEDD, "Register procd %v", sd.procdIp)
	return nil
}

func (sd *Schedd) Spawn(req proto.SpawnRequest, res *proto.SpawnResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	p := proc.MakeProcFromProto(req.ProcProto)
	db.DPrintf(db.SCHEDD, "[%v] Spawned %v", req.Realm, p)
	if _, ok := sd.qs[req.Realm]; !ok {
		sd.qs[req.Realm] = makeQueue()
	}
	// Enqueue the proc according to its realm
	sd.qs[req.Realm].Enqueue(p)
	sd.postProcInQueue(p)
	// Signal that a new proc may be runnable.
	sd.cond.Signal()
	return nil
}

// Steal a proc from this schedd.
func (sd *Schedd) StealProc(req proto.StealProcRequest, res *proto.StealProcResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// See if proc is still queued.
	var p *proc.Proc
	if p, res.OK = sd.qs[req.Realm].Steal(proc.Tpid(req.PidStr)); res.OK {
		ln := path.Join(sp.SCHEDD, req.ScheddIp, sp.QUEUE, p.GetPid().String())
		fn := path.Join(p.ParentDir, proc.WS_LINK)
		// Steal is successful. Add the new WS link to the parent's procdir.
		if _, err := sd.mfs.FsLib().PutFile(fn, 0777, sp.OWRITE, []byte(ln)); err != nil {
			db.DPrintf(db.SCHEDD_ERR, "Error write WS link", fn, err)
		}
		// Remove queue file via fslib to trigger parent watch.
		// XXX Would be nice to be able to do this in-mem too.
		if err := sd.mfs.FsLib().Remove(path.Join(sp.SCHEDD, "~local", sp.QUEUE, p.GetPid().String())); err != nil {
			db.DFatalf("Error remove %v", err)
		}
	}

	return nil
}

func (sd *Schedd) ProcDone(req proto.ProcDoneRequest, res *proto.ProcDoneResponse) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	p := proc.MakeProcFromProto(req.ProcProto)
	db.DPrintf(db.SCHEDD, "Proc done %v", p)
	sd.coresfree += p.GetNcore()
	sd.memfree += p.GetMem()
	// Signal that a new proc may be runnable.
	sd.cond.Signal()
	return nil
}

// Run a proc via the local procd.
func (sd *Schedd) runProc(p *proc.Proc) {
	sd.coresfree -= p.GetNcore()
	sd.memfree -= p.GetMem()
	// Notify schedd that the proc is done running.
	pdreq := &procdproto.RunProcRequest{
		ProcProto: p.GetProto(),
	}
	pdres := &procdproto.RunProcResponse{}
	err := sd.procd.RPC("Procd.RunProc", pdreq, pdres)
	if err != nil {
		db.DFatalf("Error RunProc schedd: %v\n%v", err, sd.qs)
	}
}

// TODO: Proper fair-share scheduling policy.
func (sd *Schedd) schedule() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	for {
		// Currently, we iterate through the realms roughly round-robin (go maps
		// are iterated in random order).
		for r, q := range sd.qs {
			// Try to dequeue a proc, whether it be from a local queue or potentially
			// stolen from a remote queue.
			if p, stolen, ok := q.Dequeue(sd.coresfree, sd.memfree); ok {
				if stolen {
					// Try to claim the proc.
					if ok := sd.stealProc(r, p); ok {
						// Proc was claimed successfully.
						db.DPrintf(db.SCHEDD, "[%v] stole proc %v", r, p)
						db.DPrintf(db.ALWAYS, "[%v] stole proc %v", r, p)
					} else {
						// Couldn't claim the proc. Move along.
						continue
					}
				}
				// Claimed a proc, so schedule it.
				db.DPrintf(db.SCHEDD, "[%v] run proc %v", r, p)
				sd.runProc(p)
				continue
			}
		}
		db.DPrintf(db.SCHEDD, "No procs runnable")
		sd.cond.Wait()
	}
}

func RunSchedd() error {
	mfs, _, _, err := memfssrv.MakeMemFs(sp.SCHEDD, sp.SCHEDDREL)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	setupFs(mfs)
	sd := MakeSchedd(mfs)
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
	return pds.RunServer()
}
