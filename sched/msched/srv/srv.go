// Package msched/srv implements the per-machine SigmaOS scheduler agent
package srv

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/proc"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	beschedclnt "sigmaos/sched/besched/clnt"
	lcschedproto "sigmaos/sched/lcsched/proto"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proto"
	"sigmaos/sched/msched/srv/procmgr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
	"sigmaos/util/syncmap"
)

type MSched struct {
	realmMu             sync.RWMutex
	mu                  sync.Mutex
	cond                *sync.Cond
	pmgr                *procmgr.ProcMgr
	mschedclnt          *mschedclnt.MSchedClnt
	beschedclnt         *beschedclnt.BESchedClnt
	pqsess              *syncmap.SyncMap[string, *BESchedSession]
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

func NewMSched(sc *sigmaclnt.SigmaClnt, kernelID string, reserveMcpu uint) *MSched {
	msched := &MSched{
		pmgr:        procmgr.NewProcMgr(sc, kernelID),
		pqsess:      syncmap.NewSyncMap[string, *BESchedSession](),
		scheddStats: make(map[sp.Trealm]*realmStats),
		mcpufree:    proc.Tmcpu(1000*linuxsched.GetNCores() - reserveMcpu),
		memfree:     mem.GetTotalMem(),
		kernelID:    kernelID,
		sc:          sc,
		cpuStats:    &cpuStats{},
	}
	msched.cond = sync.NewCond(&msched.mu)
	msched.mschedclnt = mschedclnt.NewMSchedClnt(sc.FsLib, sp.NOT_SET)
	msched.beschedclnt = beschedclnt.NewBESchedClntMSched(sc.FsLib,
		func(pqID string) {
			// When a new procq client is created, advance the epoch for the
			// corresponding procq
			pqsess, _ := msched.pqsess.AllocNew(pqID, func(pqID string) *BESchedSession {
				return NewBESchedSession(pqID, msched.kernelID)
			})
			pqsess.AdvanceEpoch()
		},
		func(pqID string) *proc.ProcSeqno {
			// Get the next proc seqno for a given procq
			pqsess, _ := msched.pqsess.AllocNew(pqID, func(pqID string) *BESchedSession {
				return NewBESchedSession(pqID, msched.kernelID)
			})
			return pqsess.NextSeqno()
		},
	)
	return msched
}

// Start procd and warm cache of binaries
func (msched *MSched) WarmProcd(ctx fs.CtxI, req proto.WarmCacheBinRequest, res *proto.WarmCacheBinResponse) error {
	if err := msched.pmgr.WarmProcd(sp.Tpid(req.PidStr), sp.Trealm(req.RealmStr), req.Program, req.SigmaPath, proc.Ttype(req.ProcType)); err != nil {
		db.DPrintf(db.ERROR, "WarmProcd %v err %v", req, err)
		res.OK = false
		return err
	}
	res.OK = true
	return nil
}

func (msched *MSched) ForceRun(ctx fs.CtxI, req proto.ForceRunRequest, res *proto.ForceRunResponse) error {
	msched.nProcsRun.Add(1)
	p := proc.NewProcFromProto(req.ProcProto)
	if ctx.Principal().GetRealm() != sp.ROOTREALM && p.GetRealm() != ctx.Principal().GetRealm() {
		return fmt.Errorf("Proc realm %v doesn't match principal realm %v", p.GetRealm(), ctx.Principal().GetRealm())
	}
	// If this proc's memory has not been accounted for (it was not spawned via
	// the BESched), account for it.
	if !req.MemAccountedFor {
		msched.allocMem(p.GetMem())
	}
	db.DPrintf(db.MSCHED, "[%v] %v ForceRun %v", p.GetRealm(), msched.kernelID, p.GetPid())
	start := time.Now()
	// Run the proc
	msched.spawnAndRunProc(p, nil)
	db.DPrintf(db.SPAWN_LAT, "[%v] MSched.ForceRun internal latency: %v", p.GetPid(), time.Since(start))
	db.DPrintf(db.MSCHED, "[%v] %v ForceRun done %v", p.GetRealm(), msched.kernelID, p.GetPid())
	return nil
}

// Wait for a proc to mark itself as started.
func (msched *MSched) WaitStart(ctx fs.CtxI, req proto.WaitRequest, res *proto.WaitResponse) error {
	db.DPrintf(db.MSCHED, "WaitStart %v seqno %v", req.PidStr, req.GetProcSeqno())
	// Wait until this schedd has heard about the proc, and has created the state
	// for it.
	if err := msched.waitUntilGotProc(req.GetProcSeqno()); err != nil {
		// XXX return in res?
		return err
	}
	msched.pmgr.WaitStart(sp.Tpid(req.PidStr))
	db.DPrintf(db.MSCHED, "WaitStart done %v", req.PidStr)
	return nil
}

// Wait for a proc to mark itself as started.
func (msched *MSched) Started(ctx fs.CtxI, req proto.NotifyRequest, res *proto.NotifyResponse) error {
	db.DPrintf(db.MSCHED, "Started %v", req.PidStr)
	start := time.Now()
	msched.pmgr.Started(sp.Tpid(req.PidStr))
	db.DPrintf(db.SPAWN_LAT, "[%v] MSched.Started internal latency: %v", req.PidStr, time.Since(start))
	return nil
}

// Wait for a proc to be evicted.
func (msched *MSched) WaitEvict(ctx fs.CtxI, req proto.WaitRequest, res *proto.WaitResponse) error {
	db.DPrintf(db.MSCHED, "WaitEvict %v", req.PidStr)
	msched.pmgr.WaitEvict(sp.Tpid(req.PidStr))
	db.DPrintf(db.MSCHED, "WaitEvict done %v", req.PidStr)
	return nil
}

// Evict a proc
func (msched *MSched) Evict(ctx fs.CtxI, req proto.NotifyRequest, res *proto.NotifyResponse) error {
	db.DPrintf(db.MSCHED, "Evict %v", req.PidStr)
	msched.pmgr.Evict(sp.Tpid(req.PidStr))
	return nil
}

// Wait for a proc to mark itself as exited.
func (msched *MSched) WaitExit(ctx fs.CtxI, req proto.WaitRequest, res *proto.WaitResponse) error {
	db.DPrintf(db.MSCHED, "WaitExit %v", req.PidStr)
	res.Status = msched.pmgr.WaitExit(sp.Tpid(req.PidStr))
	db.DPrintf(db.MSCHED, "WaitExit done %v", req.PidStr)
	return nil
}

// Wait for a proc to mark itself as exited.
func (msched *MSched) Exited(ctx fs.CtxI, req proto.NotifyRequest, res *proto.NotifyResponse) error {
	db.DPrintf(db.MSCHED, "Exited %v", req.PidStr)
	msched.pmgr.Exited(sp.Tpid(req.PidStr), req.Status)
	return nil
}

// Get CPU shares assigned to this realm.
func (msched *MSched) GetCPUShares(ctx fs.CtxI, req proto.GetCPUSharesRequest, res *proto.GetCPUSharesResponse) error {
	msched.mu.Lock()
	defer msched.mu.Unlock()

	sm := msched.pmgr.GetCPUShares()
	smap := make(map[string]int64, len(sm))
	for r, s := range sm {
		smap[r.String()] = int64(s)
	}
	res.Shares = smap
	return nil
}

// Get schedd's CPU util.
func (msched *MSched) GetCPUUtil(ctx fs.CtxI, req proto.GetCPUUtilRequest, res *proto.GetCPUUtilResponse) error {
	res.Util = msched.pmgr.GetCPUUtil(sp.Trealm(req.RealmStr))
	return nil
}

// Get realm utilization information.
func (msched *MSched) GetRunningProcs(ctx fs.CtxI, req proto.GetRunningProcsRequest, res *proto.GetRunningProcsResponse) error {
	ps := msched.pmgr.GetRunningProcs()
	res.ProcProtos = make([]*proc.ProcProto, 0, len(ps))
	for _, p := range ps {
		res.ProcProtos = append(res.ProcProtos, p.GetProto())
	}
	return nil
}

func (msched *MSched) GetMSchedStats(ctx fs.CtxI, req proto.GetMSchedStatsRequest, res *proto.GetMSchedStatsResponse) error {
	scheddStats := make(map[string]*proto.RealmStats)
	msched.realmMu.RLock()
	for r, s := range msched.scheddStats {
		st := &proto.RealmStats{
			Running:  s.running.Load(),
			TotalRan: s.totalRan.Load(),
		}
		scheddStats[r.String()] = st
	}
	msched.realmMu.RUnlock()
	res.MSchedStats = scheddStats
	return nil
}

// Note that a proc has been received and its corresponding state has been
// created, so the sequence number can be incremented
func (msched *MSched) gotProc(procSeqno *proc.ProcSeqno) {
	// schedd has successfully received a proc from procq pqID. Any clients which
	// want to wait on that proc can now expect the state for that proc to exist
	// at schedd. Set the seqno (which should be monotonically increasing) to
	// release the clients, and allow schedd to handle the wait.
	pqsess, _ := msched.pqsess.AllocNew(procSeqno.GetProcqID(), func(pqID string) *BESchedSession {
		return NewBESchedSession(pqID, msched.kernelID)
	})
	pqsess.Got(procSeqno)
}

// Wait to hear about a proc from procq pqID.
func (msched *MSched) waitUntilGotProc(pseqno *proc.ProcSeqno) error {
	// Kernel procs, spawned directly to schedd, will have an epoch of 0. Pass
	// them through (since the proc is guaranteed to have been pushed to schedd
	// by the kernel srv before calling WaitStart)
	if pseqno.GetEpoch() == 0 {
		return nil
	}
	pqsess, _ := msched.pqsess.AllocNew(pseqno.GetProcqID(), func(pqID string) *BESchedSession {
		return NewBESchedSession(pqID, msched.kernelID)
	})
	return pqsess.WaitUntilGot(pseqno)
}

// For resource accounting purposes, it is assumed that only one getQueuedProcs
// thread runs per schedd.
func (msched *MSched) getQueuedProcs() {
	// If true, bias choice of procq to this schedd's kernel.
	var bias bool = true
	for {
		memFree, ok := msched.shouldGetBEProc()
		if !ok {
			db.DPrintf(db.MSCHED, "[%v] Waiting for more mem", msched.kernelID, bias)
			// If no memory is available, wait for some more.
			msched.waitForMoreMem()
			db.DPrintf(db.MSCHED, "[%v] Waiting for mem done", msched.kernelID, bias)
			continue
		}
		db.DPrintf(db.MSCHED, "[%v] Try GetProc mem=%v bias=%v", msched.kernelID, memFree, bias)
		start := time.Now()
		// Try to get a proc from the proc queue.
		p, pseqno, qlen, ok, err := msched.beschedclnt.GetProc(msched.kernelID, memFree, bias)
		var pmem proc.Tmem
		if ok {
			pmem = p.GetMem()
		}
		db.DPrintf(db.MSCHED, "[%v] GetProc result pseqno %v procMem %v qlen %v ok %v", msched.kernelID, pseqno, pmem, qlen, ok)
		if ok {
			db.DPrintf(db.SPAWN_LAT, "GetProc latency: %v", time.Since(start))
		} else {
			db.DPrintf(db.SPAWN_LAT, "GetProc timeout")
		}
		if err != nil {
			db.DPrintf(db.MSCHED_ERR, "Error GetProc: %v", err)
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
		msched.nProcGets.Add(1)
		if !ok {
			db.DPrintf(db.MSCHED, "[%v] No proc on procq, try another, bias=%v qlen=%v", msched.kernelID, bias, qlen)
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
		msched.nProcGetsSuccessful.Add(1)
		// Allocate memory for the proc before this loop runs again so that
		// subsequent getProc requests carry the updated memory accounting
		// information.
		msched.allocMem(p.GetMem())
		db.DPrintf(db.MSCHED, "[%v] Got proc [%v] from procq, bias=%v", msched.kernelID, p.GetPid(), bias)
		// Run the proc
		msched.spawnAndRunProc(p, pseqno)
	}
}

func (msched *MSched) procDone(p *proc.Proc) {
	db.DPrintf(db.MSCHED, "Proc done %v", p)
	// Free any mem the proc was using.
	msched.freeMem(p.GetMem())
}

func (msched *MSched) spawnAndRunProc(p *proc.Proc, pseqno *proc.ProcSeqno) {
	db.DPrintf(db.MSCHED, "spawnAndRunProc %v", p)
	// Free any mem the proc was using.
	msched.incRealmStats(p)
	p.SetKernelID(msched.kernelID, false)
	msched.pmgr.Spawn(p)
	// If this proc was spawned via procq, handle the sequence number
	if pseqno != nil {
		// Proc state now exists. Mark it as such to release any clients which may
		// be waiting in WaitStart for schedd to receive this proc
		msched.gotProc(pseqno)
	}
	// Run the proc
	go msched.runProc(p)
}

// Run a proc via the local schedd. Caller holds lock.
func (msched *MSched) runProc(p *proc.Proc) {
	defer msched.decRealmStats(p)
	db.DPrintf(db.MSCHED, "[%v] %v runProc %v", p.GetRealm(), msched.kernelID, p)
	msched.pmgr.RunProc(p)
	msched.procDone(p)
}

// We should always take a free proc if there is memory available.
func (msched *MSched) shouldGetBEProc() (proc.Tmem, bool) {
	mem := msched.getFreeMem()
	cpu := msched.getCPUUtil()
	db.DPrintf(db.MSCHED, "CPU util check: %v", cpu)
	return mem, mem > 0 && cpu < (sp.Conf.MSched.TARGET_CPU_UTIL*int64(linuxsched.GetNCores()))
}

func (msched *MSched) register() {
	rpcc, err := sprpcclnt.NewRPCClnt(msched.sc.FsLib, filepath.Join(sp.LCSCHED, sp.ANY))
	if err != nil {
		db.DFatalf("Error lsched rpccc: %v", err)
	}
	req := &lcschedproto.RegisterMSchedRequest{
		KernelID: msched.kernelID,
		McpuInt:  uint32(msched.mcpufree),
		MemInt:   uint32(msched.memfree),
	}
	res := &lcschedproto.RegisterMSchedResponse{}
	if err := rpcc.RPC("LCSched.RegisterMSched", req, res); err != nil {
		db.DFatalf("Error LCSched RegisterMSched: %v", err)
	}
}

func (msched *MSched) stats() {
	if !db.WillBePrinted(db.MSCHED) {
		return
	}
	for {
		time.Sleep(time.Second)
		db.DPrintf(db.ALWAYS, "nget %v successful %v", msched.nProcGets.Load(), msched.nProcGetsSuccessful.Load())
	}
}

func RunMSched(kernelID string, reserveMcpu uint) error {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	sc.GetDialProxyClnt().AllowConnectionsFromAllRealms()
	msched := NewMSched(sc, kernelID, reserveMcpu)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.MSCHED, kernelID), sc, msched)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	msched.pmgr.SetMemFs(ssrv.MemFs)
	// Perf monitoring
	p, err := perf.NewPerf(sc.ProcEnv(), perf.MSCHED)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	db.DPrintf(db.ALWAYS, "MSched starting with total mem: %v", mem.GetTotalMem())
	defer p.Done()
	go msched.getQueuedProcs()
	go msched.stats()
	go msched.monitorCPU()
	msched.register()
	ssrv.RunServer()
	return nil
}
