package schedd

import (
	"path"
	"sync"
	"sync/atomic"
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
	realmMu             sync.RWMutex
	mu                  sync.Mutex
	cond                *sync.Cond
	pmgr                *procmgr.ProcMgr
	scheddclnt          *scheddclnt.ScheddClnt
	procqclnt           *procqclnt.ProcQClnt
	mcpufree            proc.Tmcpu
	memfree             proc.Tmem
	kernelId            string
	scheddStats         map[sp.Trealm]*proto.RealmStats
	mfs                 *memfssrv.MemFs
	cpuStats            *cpuStats
	cpuUtil             int64
	nProcsRun           uint64
	nProcGets           uint64
	nProcGetsSuccessful uint64
}

func NewSchedd(mfs *memfssrv.MemFs, kernelId string, reserveMcpu uint) *Schedd {
	sd := &Schedd{
		pmgr:        procmgr.NewProcMgr(mfs, kernelId),
		scheddStats: make(map[sp.Trealm]*proto.RealmStats),
		mcpufree:    proc.Tmcpu(1000*linuxsched.GetNCores() - reserveMcpu),
		memfree:     mem.GetTotalMem(),
		kernelId:    kernelId,
		mfs:         mfs,
		cpuStats:    &cpuStats{},
	}
	sd.cond = sync.NewCond(&sd.mu)
	sd.scheddclnt = scheddclnt.NewScheddClnt(mfs.SigmaClnt().FsLib)
	sd.procqclnt = procqclnt.NewProcQClnt(mfs.SigmaClnt().FsLib)
	return sd
}

// Warm the cache of proc binaries.
func (sd *Schedd) WarmCacheBin(ctx fs.CtxI, req proto.WarmCacheBinRequest, res *proto.WarmCacheBinResponse) error {
	if err := sd.pmgr.DownloadProcBin(sp.Trealm(req.RealmStr), req.Program, req.BuildTag, proc.Ttype(req.ProcType)); err != nil {
		db.DFatalf("Error Download Proc Bin: %v", err)
		res.OK = false
		return err
	}
	res.OK = true
	return nil
}

func (sd *Schedd) ForceRun(ctx fs.CtxI, req proto.ForceRunRequest, res *proto.ForceRunResponse) error {
	atomic.AddUint64(&sd.nProcsRun, 1)
	p := proc.NewProcFromProto(req.ProcProto)
	// If this proc's memory has not been accounted for (it was not spawned via
	// the ProcQ), account for it.
	if !req.MemAccountedFor {
		sd.allocMem(p.GetMem())
	}
	sd.incRealmStats(p)
	db.DPrintf(db.SCHEDD, "[%v] %v ForceRun %v", p.GetRealm(), sd.kernelId, p.GetPid())
	start := time.Now()
	// Run the proc
	sd.spawnAndRunProc(p)
	db.DPrintf(db.SPAWN_LAT, "[%v] Schedd.ForceRun internal latency: %v", p.GetPid(), time.Since(start))
	db.DPrintf(db.SCHEDD, "[%v] %v ForceRun done %v", p.GetRealm(), sd.kernelId, p.GetPid())
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
	res.Status = sd.pmgr.WaitExit(sp.Tpid(req.PidStr))
	db.DPrintf(db.SCHEDD, "WaitExit done %v", req.PidStr)
	return nil
}

// Wait for a proc to mark itself as exited.
func (sd *Schedd) Exited(ctx fs.CtxI, req proto.NotifyRequest, res *proto.NotifyResponse) error {
	db.DPrintf(db.SCHEDD, "Exited %v", req.PidStr)
	sd.pmgr.Exited(sp.Tpid(req.PidStr), req.Status)
	return nil
}

func (sd *Schedd) CheckpointProc(ctx fs.CtxI, req proto.CheckpointProcRequest, res *proto.CheckpointProcResponse) error {
	p := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.SCHEDD, "CheckpointProc %v", req.ProcProto.ProcEnvProto.PidStr)
	chkptLoc, osPid, err := sd.pmgr.CheckpointProc(p)
	res.CheckpointLocation = chkptLoc
	res.OsPid = int32(osPid)
	db.DPrintf(db.SCHEDD, "CheckpointProc done %v", req.ProcProto.ProcEnvProto.PidStr)
	if err != nil {
		return err
	}
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

// Get realm utilization information.
func (sd *Schedd) GetRunningProcs(ctx fs.CtxI, req proto.GetRunningProcsRequest, res *proto.GetRunningProcsResponse) error {
	ps := sd.pmgr.GetRunningProcs()
	res.ProcProtos = make([]*proc.ProcProto, 0, len(ps))
	for _, p := range ps {
		res.ProcProtos = append(res.ProcProtos, p.GetProto())
	}
	return nil
}

func (sd *Schedd) GetScheddStats(ctx fs.CtxI, req proto.GetScheddStatsRequest, res *proto.GetScheddStatsResponse) error {
	scheddStats := make(map[string]*proto.RealmStats)
	sd.realmMu.RLock()
	for r, s := range sd.scheddStats {
		st := &proto.RealmStats{
			Running:  atomic.LoadInt64(&s.Running),
			TotalRan: atomic.LoadInt64(&s.TotalRan),
		}
		scheddStats[r.String()] = st
	}
	sd.realmMu.RUnlock()
	res.ScheddStats = scheddStats
	return nil
}

// For resource accounting purposes, it is assumed that only one getQueuedProcs
// thread runs per schedd.
func (sd *Schedd) getQueuedProcs() {
	// If true, bias choice of procq to this schedd's kernel.
	var bias bool = true
	for {
		memFree, ok := sd.shouldGetBEProc()
		if !ok {
			db.DPrintf(db.SCHEDD, "[%v] Waiting for more mem", sd.kernelId, bias)
			// If no memory is available, wait for some more.
			sd.waitForMoreMem()
			db.DPrintf(db.SCHEDD, "[%v] Waiting for mem done", sd.kernelId, bias)
			continue
		}
		db.DPrintf(db.SCHEDD, "[%v] Try GetProc mem=%v bias=%v", sd.kernelId, memFree, bias)
		start := time.Now()
		// Try to get a proc from the proc queue.
		procMem, qlen, ok, err := sd.procqclnt.GetProc(sd.kernelId, memFree, bias)
		db.DPrintf(db.SCHEDD, "[%v] GetProc result procMem %v qlen %v ok %v", sd.kernelId, procMem, qlen, ok)
		db.DPrintf(db.SPAWN_LAT, "GetProc latency: %v", time.Since(start))
		if err != nil {
			db.DPrintf(db.SCHEDD_ERR, "Error GetProc: %v", err)
			// If previously biased to this schedd's kernel, and GetProc returned an
			// error, then un-bias.
			//
			// If not biased to this schedd's kernel, and GetProc returned an error,
			// then bias on the next attempt.
			if bias {
				bias = false
			} else {
				bias = true
			}
			continue
		}
		atomic.AddUint64(&sd.nProcGets, 1)
		if !ok {
			db.DPrintf(db.SCHEDD, "[%v] No proc on procq, try another, bias=%v qlen=%v", sd.kernelId, bias, qlen)
			// If already biased to this schedd's kernel, and no proc was available,
			// try another.
			//
			// If not biased to this schedd's kernel, and no proc was available, then
			// bias on the next attempt.
			if bias {
				bias = false
			} else {
				bias = true
			}
			continue
		}
		// Restore bias if successful (since getProc may have been unbiased and led
		// to a successful claim before)
		bias = true
		atomic.AddUint64(&sd.nProcGetsSuccessful, 1)
		// Allocate memory for the proc before this loop runs again so that
		// subsequent getProc requests carry the updated memory accounting
		// information.
		sd.allocMem(procMem)
		db.DPrintf(db.SCHEDD, "[%v] Got proc from procq, bias=%v", sd.kernelId, bias)
	}
}

func (sd *Schedd) procDone(p *proc.Proc) {
	db.DPrintf(db.SCHEDD, "Proc done %v", p)
	// Free any mem the proc was using.
	sd.freeMem(p.GetMem())
}

func (sd *Schedd) spawnAndRunProc(p *proc.Proc) {
	p.SetKernelID(sd.kernelId, false)
	sd.pmgr.Spawn(p)
	// Run the proc
	go sd.runProc(p)
}

// Run a proc via the local procd. Caller holds lock.
func (sd *Schedd) runProc(p *proc.Proc) {
	defer sd.decRealmStats(p)
	db.DPrintf(db.SCHEDD, "[%v] %v runProc %v", p.GetRealm(), sd.kernelId, p)
	sd.pmgr.RunProc(p)
	sd.procDone(p)
}

// We should always take a free proc if there is memory available.
func (sd *Schedd) shouldGetBEProc() (proc.Tmem, bool) {
	mem := sd.getFreeMem()
	cpu := sd.getCPUUtil()
	db.DPrintf(db.SCHEDD, "CPU util check: %v", cpu)
	return mem, mem > 0 && cpu < (TARGET_CPU_PCT*int64(linuxsched.GetNCores()))
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

func (sd *Schedd) stats() {
	if !db.WillBePrinted(db.SCHEDD) {
		return
	}
	for {
		time.Sleep(time.Second)
		db.DPrintf(db.ALWAYS, "nget %v successful %v", atomic.LoadUint64(&sd.nProcGets), atomic.LoadUint64(&sd.nProcGetsSuccessful))
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
	sd.pmgr.SetupFs(ssrv.MemFs)
	// Perf monitoring
	p, err := perf.NewPerf(pcfg, perf.SCHEDD)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	db.DPrintf(db.ALWAYS, "Schedd starting with total mem: %v", mem.GetTotalMem())
	defer p.Done()
	go sd.getQueuedProcs()
	go sd.stats()
	go sd.monitorCPU()
	sd.register()
	ssrv.RunServer()
	return nil
}
