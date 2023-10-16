package schedd

import (
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	lcproto "sigmaos/lcschedsrv/proto"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procmgr"
	"sigmaos/procqclnt"
	"sigmaos/rpcclnt"
	"sigmaos/schedd/proto"
	"sigmaos/scheddclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type Schedd struct {
	mu         sync.Mutex
	cond       *sync.Cond
	pmgr       *procmgr.ProcMgr
	scheddclnt *scheddclnt.ScheddClnt
	procqclnt  *procqclnt.ProcQClnt
	mcpufree   proc.Tmcpu
	memfree    proc.Tmem
	kernelId   string
	realms     []sp.Trealm
	mfs        *memfssrv.MemFs
}

func NewSchedd(mfs *memfssrv.MemFs, kernelId string, reserveMcpu uint) *Schedd {
	sd := &Schedd{
		pmgr:     procmgr.NewProcMgr(mfs, kernelId),
		realms:   make([]sp.Trealm, 0),
		mcpufree: proc.Tmcpu(1000*linuxsched.GetNCores() - reserveMcpu),
		memfree:  mem.GetTotalMem(),
		kernelId: kernelId,
		mfs:      mfs,
	}
	sd.cond = sync.NewCond(&sd.mu)
	sd.scheddclnt = scheddclnt.NewScheddClnt(mfs.SigmaClnt().FsLib)
	sd.procqclnt = procqclnt.NewProcQClnt(mfs.SigmaClnt().FsLib)
	return sd
}

func (sd *Schedd) ForceRun(ctx fs.CtxI, req proto.ForceRunRequest, res *proto.ForceRunResponse) error {
	p := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.SCHEDD, "[%v] %v ForceRun %v", p.GetRealm(), sd.kernelId, p)
	// Run the proc
	go sd.spawnAndRunProc(p)
	return nil
}

// Wait for a proc to mark itself as started.
func (sd *Schedd) WaitStart(ctx fs.CtxI, req proto.WaitRequest, res *proto.WaitResponse) error {
	db.DPrintf(db.SCHEDD, "WaitStart %v", req.PidStr)
	sd.pmgr.WaitStart(sp.Tpid(req.PidStr))
	db.DPrintf(db.SCHEDD, "WaitStart done %v", req.PidStr)
	return nil
}

// Wait for a proc to mark itself as started.
func (sd *Schedd) Started(ctx fs.CtxI, req proto.NotifyRequest, res *proto.NotifyResponse) error {
	db.DPrintf(db.SCHEDD, "Started %v", req.PidStr)
	start := time.Now()
	sd.pmgr.Started(sp.Tpid(req.PidStr))
	db.DPrintf(db.SPAWN_LAT, "[%v] Schedd.Started internal latency: %v", req.PidStr, time.Since(start))
	return nil
}

// Wait for a proc to be evicted.
func (sd *Schedd) WaitEvict(ctx fs.CtxI, req proto.WaitRequest, res *proto.WaitResponse) error {
	db.DPrintf(db.SCHEDD, "WaitEvict %v", req.PidStr)
	sd.pmgr.WaitEvict(sp.Tpid(req.PidStr))
	db.DPrintf(db.SCHEDD, "WaitEvict done %v", req.PidStr)
	return nil
}

// Wait for a proc to mark itself as exited.
func (sd *Schedd) Evict(ctx fs.CtxI, req proto.NotifyRequest, res *proto.NotifyResponse) error {
	db.DPrintf(db.SCHEDD, "Evict %v", req.PidStr)
	sd.pmgr.Evict(sp.Tpid(req.PidStr))
	return nil
}

// Wait for a proc to mark itself as exited.
func (sd *Schedd) WaitExit(ctx fs.CtxI, req proto.WaitRequest, res *proto.WaitResponse) error {
	db.DPrintf(db.SCHEDD, "WaitExit %v", req.PidStr)
	status := sd.pmgr.WaitExit(sp.Tpid(req.PidStr))
	res.Status = status.Marshal()
	db.DPrintf(db.SCHEDD, "WaitExit done %v", req.PidStr)
	return nil
}

// Wait for a proc to mark itself as exited.
func (sd *Schedd) Exited(ctx fs.CtxI, req proto.NotifyRequest, res *proto.NotifyResponse) error {
	db.DPrintf(db.SCHEDD, "Exited %v", req.PidStr)
	status := proc.NewStatusFromBytes(req.Status)
	sd.pmgr.Exited(sp.Tpid(req.PidStr), status)
	return nil
}

// Get CPU shares assigned to this realm.
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

// Get schedd's CPU util.
func (sd *Schedd) GetCPUUtil(ctx fs.CtxI, req proto.GetCPUUtilRequest, res *proto.GetCPUUtilResponse) error {
	res.Util = sd.pmgr.GetCPUUtil(sp.Trealm(req.RealmStr))
	return nil
}

func (sd *Schedd) getQueuedProcs() {
	for {
		if sd.shouldGetProc() {
		}
		db.DPrintf(db.SCHEDD, "[%v] Try get proc from procq", sd.kernelId)
		start := time.Now()
		// Try to get a proc from the proc queue.
		ok, err := sd.procqclnt.GetProc(sd.kernelId)
		db.DPrintf(db.SPAWN_LAT, "GetProc latency: %v", time.Since(start))
		if err != nil {
			db.DPrintf(db.SCHEDD_ERR, "Error GetProc: %v", err)
			continue
		}
		if !ok {
			db.DPrintf(db.SCHEDD, "[%v] No proc on procq, try another", sd.kernelId)
			continue
		}
		db.DPrintf(db.SCHEDD, "[%v] Got proc from procq", sd.kernelId)
	}
}

func (sd *Schedd) procDone(p *proc.Proc) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	db.DPrintf(db.SCHEDD, "Proc done %v", p)
	// Signal that a new proc may be runnable.
	sd.cond.Signal()
	return nil
}

func (sd *Schedd) spawnAndRunProc(p *proc.Proc) {
	p.SetKernelID(sd.kernelId, false)
	sd.pmgr.Spawn(p)
	// Run the proc
	go sd.runProc(p)
}

// Run a proc via the local procd. Caller holds lock.
func (sd *Schedd) runProc(p *proc.Proc) {
	db.DPrintf(db.SCHEDD, "[%v] %v runProc %v", p.GetRealm(), sd.kernelId, p)
	sd.pmgr.RunProc(p)
	sd.procDone(p)
}

func (sd *Schedd) shouldGetProc() bool {
	// TODO: check local resource utilization
	return true
}

func (sd *Schedd) register() {
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{sd.mfs.SigmaClnt().FsLib}, path.Join(sp.LCSCHED, "~any"))
	if err != nil {
		db.DFatalf("Error lsched rpccc: %v", err)
	}
	req := &lcproto.RegisterScheddRequest{
		KernelID: sd.kernelId,
		McpuInt:  uint32(sd.mcpufree),
		MemInt:   uint32(sd.memfree),
	}
	res := &lcproto.RegisterScheddResponse{}
	if err := rpcc.RPC("LCSched.RegisterSchedd", req, res); err != nil {
		db.DFatalf("Error LCSched RegisterSchedd: %v", err)
	}
}

func RunSchedd(kernelId string, reserveMcpu uint) error {
	pcfg := proc.GetProcEnv()
	mfs, err := memfssrv.NewMemFs(path.Join(sp.SCHEDD, kernelId), pcfg)
	if err != nil {
		db.DFatalf("Error NewMemFs: %v", err)
	}
	sd := NewSchedd(mfs, kernelId, reserveMcpu)
	ssrv, err := sigmasrv.NewSigmaSrvMemFs(mfs, sd)
	if err != nil {
		db.DFatalf("Error PDS: %v", err)
	}
	setupMemFsSrv(ssrv.MemFs)
	setupFs(ssrv.MemFs)
	// Perf monitoring
	p, err := perf.NewPerf(pcfg, perf.SCHEDD)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()
	go sd.getQueuedProcs()
	sd.register()
	ssrv.RunServer()
	return nil
}
