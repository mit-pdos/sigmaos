package schedd

import (
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
)

type Schedd struct {
	sync.Mutex
	procdIp string
	procd   *protdevclnt.ProtDevClnt
	mfs     *memfssrv.MemFs
	qs      map[string]*Queue
}

func MakeSchedd(mfs *memfssrv.MemFs) *Schedd {
	return &Schedd{
		mfs: mfs,
		qs:  make(map[string]*Queue),
	}
}

func (sd *Schedd) RegisterProcd(req proto.RegisterRequest, res *proto.RegisterResponse) error {
	if sd.procdIp != "" {
		db.DFatalf("Register procd on schedd with procd already registered")
	}
	sd.procdIp = req.ProcdIp
	var err error
	sd.procd, err = protdevclnt.MkProtDevClnt(sd.mfs.FsLib(), path.Join(sp.PROCD, sd.procdIp))
	if err != nil {
		db.DFatalf("Error make procd clnt: %v", err)
	}
	return nil
}

func (sd *Schedd) Spawn(req proto.SpawnRequest, res *proto.SpawnResponse) error {
	sd.Lock()
	defer sd.Unlock()

	p := proc.MakeProcFromProto(req.ProcProto)

	db.DPrintf(db.SCHEDD, "[%v] Spawned %v", req.Realm, p)
	if _, ok := sd.qs[req.Realm]; !ok {
		sd.qs[req.Realm] = makeQueue()
	}

	sd.qs[req.Realm].Enqueue(p)
	if _, err := sd.mfs.Create(path.Join(sp.QUEUE, p.GetPid().String()), 0777, sp.OWRITE); err != nil {
		db.DFatalf("Error create %v: %v", p.GetPid(), err)
	}

	//	// XXX For now, immediately dequeue the proc and spawn it. Of course, this
	//	// will be done according to heuristics and resource utilization in future.
	//	var ok bool
	//	var pproto *proc.ProcProto
	//	if pproto, ok = sd.qs[req.Realm].Dequeue(); !ok {
	//		db.DFatalf("Couldn't dequeue enqueued proc")
	//	}
	//	p = proc.MakeProcFromProto(pproto)

	// TODO: RPC to procd.

	return nil
}

func (sd *Schedd) GetProc(req proto.GetProcRequest, res *proto.GetProcResponse) error {
	// TODO: choose a realm in a more sensible way.
	for _, q := range sd.qs {
		// TODO: check capacity
		if res.ProcProto, res.OK = q.Dequeue(); res.OK {
			break
		}
	}
	return nil
}

// Setup schedd's fs.
func setupFs(mfs *memfssrv.MemFs) {
	dirs := []string{
		sp.QUEUE,
	}
	for _, d := range dirs {
		if _, err := mfs.Create(d, sp.DMDIR|0777, sp.OWRITE); err != nil {
			db.DFatalf("Error create %v: %v", d, err)
		}
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
	return pds.RunServer()
}
