package schedsrv

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"sigmaos/beschedclnt"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	lcproto "sigmaos/lcschedsrv/proto"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/util/perf"
	"sigmaos/proc"
	"sigmaos/scheddclnt"
	"sigmaos/schedsrv/procmgr"
	"sigmaos/schedsrv/proto"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
	"sigmaos/sigmasrv"
	"sigmaos/util/syncmap"
)

type Schedd struct {
	realmMu             sync.RWMutex
	mu                  sync.Mutex
	cond                *sync.Cond
	pmgr                *procmgr.ProcMgr
	scheddclnt          *scheddclnt.ScheddClnt
	beschedclnt         *beschedclnt.BESchedClnt
	pqsess              *syncmap.SyncMap[string, *ProcqSession]
	mcpufree            proc.Tmcpu
	memfree             proc.Tmem
	kernelID            string
	scheddStats         map[sp.Trealm]*realmStats
	sc                  *sigmaclnt.SigmaClnt
	cpuStats            *cpuStats
	cpuUtil             int64
	nProcsRun           atomic.Uint64
	nProcGets           atomic.Uint64
	nProcGetsSuccessful atomic.Uint64
}

func NewSchedd(sc *sigmaclnt.SigmaClnt, kernelID string, reserveMcpu uint) *Schedd {
	sd := &Schedd{
		pmgr:        procmgr.NewProcMgr(sc, kernelID),
		pqsess:      syncmap.NewSyncMap[string, *ProcqSession](),
		scheddStats: make(map[sp.Trealm]*realmStats),
		mcpufree:    proc.Tmcpu(1000*linuxsched.GetNCores() - reserveMcpu),
		memfree:     mem.GetTotalMem(),
		kernelID:    kernelID,
		sc:          sc,
		cpuStats:    &cpuStats{},
	}
	sd.cond = sync.NewCond(&sd.mu)
	sd.scheddclnt = scheddclnt.NewScheddClnt(sc.FsLib, sp.NOT_SET)
	sd.beschedclnt = beschedclnt.NewBESchedClntSchedd(sc.FsLib,
		func(pqID string) {
			// When a new procq client is created, advance the epoch for the
			// corresponding procq
			pqsess, _ := sd.pqsess.AllocNew(pqID, func(pqID string) *ProcqSession {
				return NewProcqSession(pqID, sd.kernelID)
			})
			pqsess.AdvanceEpoch()
		},
		func(pqID string) *proc.ProcSeqno {
			// Get the next proc seqno for a given procq
			pqsess, _ := sd.pqsess.AllocNew(pqID, func(pqID string) *ProcqSession {
				return NewProcqSession(pqID, sd.kernelID)
			})
			return pqsess.NextSeqno()
		},
	)
	return sd
}

// Start uprocd and warm cache of binaries
func (sd *Schedd) WarmUprocd(ctx fs.CtxI, req proto.WarmCacheBinRequest, res *proto.WarmCacheBinResponse) error {
	if err := sd.pmgr.WarmUprocd(sp.Tpid(req.PidStr), sp.Trealm(req.RealmStr), req.Program, req.SigmaPath, proc.Ttype(req.ProcType)); err != nil {
		db.DPrintf(db.ERROR, "WarmUprocd %v err %v", req, err)
		res.OK = false
		return err
	}
	res.OK = true
	return nil
}

func (sd *Schedd) ForceRun(ctx fs.CtxI, req proto.ForceRunRequest, res *proto.ForceRunResponse) error {
	sd.nProcsRun.Add(1)
	p := proc.NewProcFromProto(req.ProcProto)
	if ctx.Principal().GetRealm() != sp.ROOTREALM && p.GetRealm() != ctx.Principal().GetRealm() {
		return fmt.Errorf("Proc realm %v doesn't match principal realm %v", p.GetRealm(), ctx.Principal().GetRealm())
	}
	// If this proc's memory has not been accounted for (it was not spawned via
	// the BESched), account for it.
	if !req.MemAccountedFor {
		sd.allocMem(p.GetMem())
	}
	db.DPrintf(db.SCHEDD, "[%v] %v ForceRun %v", p.GetRealm(), sd.kernelID, p.GetPid())
	start := time.Now()
	// Run the proc
	sd.spawnAndRunProc(p, nil)
	db.DPrintf(db.SPAWN_LAT, "[%v] Schedd.ForceRun internal latency: %v", p.GetPid(), time.Since(start))
	db.DPrintf(db.SCHEDD, "[%v] %v ForceRun done %v", p.GetRealm(), sd.kernelID, p.GetPid())
	return nil
}

// Wait for a proc to mark itself as started.
func (sd *Schedd) WaitStart(ctx fs.CtxI, req proto.WaitRequest, res *proto.WaitResponse) error {
	db.DPrintf(db.SCHEDD, "WaitStart %v seqno %v", req.PidStr, req.GetProcSeqno())
	// Wait until this schedd has heard about the proc, and has created the state
	// for it.
	if err := sd.waitUntilGotProc(req.GetProcSeqno()); err != nil {
		// XXX return in res?
		return err
	}
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

// Evict a proc
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
			Running:  s.running.Load(),
			TotalRan: s.totalRan.Load(),
		}
		scheddStats[r.String()] = st
	}
	sd.realmMu.RUnlock()
	res.ScheddStats = scheddStats
	return nil
}

// Note that a proc has been received and its corresponding state has been
// created, so the sequence number can be incremented
func (sd *Schedd) gotProc(procSeqno *proc.ProcSeqno) {
	// schedd has successfully received a proc from procq pqID. Any clients which
	// want to wait on that proc can now expect the state for that proc to exist
	// at schedd. Set the seqno (which should be monotonically increasing) to
	// release the clients, and allow schedd to handle the wait.
	pqsess, _ := sd.pqsess.AllocNew(procSeqno.GetProcqID(), func(pqID string) *ProcqSession {
		return NewProcqSession(pqID, sd.kernelID)
	})
	pqsess.Got(procSeqno)
}

// Wait to hear about a proc from procq pqID.
func (sd *Schedd) waitUntilGotProc(pseqno *proc.ProcSeqno) error {
	// Kernel procs, spawned directly to schedd, will have an epoch of 0. Pass
	// them through (since the proc is guaranteed to have been pushed to schedd
	// by the kernel srv before calling WaitStart)
	if pseqno.GetEpoch() == 0 {
		return nil
	}
	pqsess, _ := sd.pqsess.AllocNew(pseqno.GetProcqID(), func(pqID string) *ProcqSession {
		return NewProcqSession(pqID, sd.kernelID)
	})
	return pqsess.WaitUntilGot(pseqno)
}

// For resource accounting purposes, it is assumed that only one getQueuedProcs
// thread runs per schedd.
func (sd *Schedd) getQueuedProcs() {
	// If true, bias choice of procq to this schedd's kernel.
	var bias bool = true
	for {
		memFree, ok := sd.shouldGetBEProc()
		if !ok {
			db.DPrintf(db.SCHEDD, "[%v] Waiting for more mem", sd.kernelID, bias)
			// If no memory is available, wait for some more.
			sd.waitForMoreMem()
			db.DPrintf(db.SCHEDD, "[%v] Waiting for mem done", sd.kernelID, bias)
			continue
		}
		db.DPrintf(db.SCHEDD, "[%v] Try GetProc mem=%v bias=%v", sd.kernelID, memFree, bias)
		start := time.Now()
		// Try to get a proc from the proc queue.
		p, pseqno, qlen, ok, err := sd.beschedclnt.GetProc(sd.kernelID, memFree, bias)
		var pmem proc.Tmem
		if ok {
			pmem = p.GetMem()
		}
		db.DPrintf(db.SCHEDD, "[%v] GetProc result pseqno %v procMem %v qlen %v ok %v", sd.kernelID, pseqno, pmem, qlen, ok)
		if ok {
			db.DPrintf(db.SPAWN_LAT, "GetProc latency: %v", time.Since(start))
		} else {
			db.DPrintf(db.SPAWN_LAT, "GetProc timeout")
		}
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
		sd.nProcGets.Add(1)
		if !ok {
			db.DPrintf(db.SCHEDD, "[%v] No proc on procq, try another, bias=%v qlen=%v", sd.kernelID, bias, qlen)
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
		sd.nProcGetsSuccessful.Add(1)
		// Allocate memory for the proc before this loop runs again so that
		// subsequent getProc requests carry the updated memory accounting
		// information.
		sd.allocMem(p.GetMem())
		db.DPrintf(db.SCHEDD, "[%v] Got proc [%v] from procq, bias=%v", sd.kernelID, p.GetPid(), bias)
		// Run the proc
		sd.spawnAndRunProc(p, pseqno)
	}
}

func (sd *Schedd) procDone(p *proc.Proc) {
	db.DPrintf(db.SCHEDD, "Proc done %v", p)
	// Free any mem the proc was using.
	sd.freeMem(p.GetMem())
}

func (sd *Schedd) spawnAndRunProc(p *proc.Proc, pseqno *proc.ProcSeqno) {
	db.DPrintf(db.SCHEDD, "spawnAndRunProc %v", p)
	// Free any mem the proc was using.
	sd.incRealmStats(p)
	p.SetKernelID(sd.kernelID, false)
	sd.pmgr.Spawn(p)
	// If this proc was spawned via procq, handle the sequence number
	if pseqno != nil {
		// Proc state now exists. Mark it as such to release any clients which may
		// be waiting in WaitStart for schedd to receive this proc
		sd.gotProc(pseqno)
	}
	// Run the proc
	go sd.runProc(p)
}

// Run a proc via the local schedd. Caller holds lock.
func (sd *Schedd) runProc(p *proc.Proc) {
	defer sd.decRealmStats(p)
	db.DPrintf(db.SCHEDD, "[%v] %v runProc %v", p.GetRealm(), sd.kernelID, p)
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
	rpcc, err := sigmarpcchan.NewSigmaRPCClnt([]*fslib.FsLib{sd.sc.FsLib}, filepath.Join(sp.LCSCHED, sp.ANY))
	if err != nil {
		db.DFatalf("Error lsched rpccc: %v", err)
	}
	req := &lcproto.RegisterScheddRequest{
		KernelID: sd.kernelID,
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
		db.DPrintf(db.ALWAYS, "nget %v successful %v", sd.nProcGets.Load(), sd.nProcGetsSuccessful.Load())
	}
}

func RunSchedd(kernelID string, reserveMcpu uint) error {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	sc.GetNetProxyClnt().AllowConnectionsFromAllRealms()
	sd := NewSchedd(sc, kernelID, reserveMcpu)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.SCHEDD, kernelID), sc, sd)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	if err := sd.pmgr.SetupFs(ssrv.MemFs); err != nil {
		db.DFatalf("Error SetupFs: %v", err)
	}
	// Perf monitoring
	p, err := perf.NewPerf(sc.ProcEnv(), perf.SCHEDD)
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
