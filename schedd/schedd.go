package schedd

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/protdevsrv"
	"sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
)

type Schedd struct {
	sync.Mutex
	qs map[string]*Queue
}

func MakeSchedd() *Schedd {
	return &Schedd{
		qs: make(map[string]*Queue),
	}
}

func (sd *Schedd) Spawn(req proto.SpawnRequest, res *proto.SpawnResponse) error {
	sd.Lock()
	defer sd.Unlock()

	db.DPrintf(db.SCHEDD, "[%v] Spawned %v", req.Realm, req.ProcStr)
	if _, ok := sd.qs[req.Realm]; !ok {
		sd.qs[req.Realm] = makeQueue()
	}

	sd.qs[req.Realm].Enqueue(req.ProcStr)

	res.OK = true
	return nil
}

func RunSchedd() error {
	sd := MakeSchedd()
	mfs, _, _, err := memfssrv.MakeMemFs(sp.SCHEDD, sp.SCHEDDREL)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
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
