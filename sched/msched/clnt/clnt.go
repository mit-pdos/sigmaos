package clnt

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	shardedsvcrpcclnt "sigmaos/rpc/shardedsvc/clnt"
	"sigmaos/sched/msched/proto"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

type MSchedClnt struct {
	mu sync.Mutex
	*fslib.FsLib
	rpcdc *shardedsvcrpcclnt.ShardedSvcRPCClnt
	done  int32

	// rpcclnt for my msched
	kernelID string
	rpcc     *rpcclnt.RPCClnt
}

func NewMSchedClnt(fsl *fslib.FsLib, kernelID string) *MSchedClnt {
	return &MSchedClnt{
		FsLib:    fsl,
		kernelID: kernelID,
		rpcdc:    shardedsvcrpcclnt.NewShardedSvcRPCClnt(fsl, sp.MSCHED, db.MSCHEDCLNT, db.MSCHEDCLNT_ERR),
	}
}

func (mc *MSchedClnt) NMSched() (int, error) {
	return mc.rpcdc.Nentry()
}

func (mc *MSchedClnt) GetMScheds() ([]string, error) {
	return mc.rpcdc.WaitGetEntriesN(1, true)
}

func (mc *MSchedClnt) UnregisterSrv(mschedID string) {
	mc.rpcdc.InvalidateEntry(mschedID)
}

func (mc *MSchedClnt) Nprocs(procdir string) (int, error) {
	sts, err := mc.GetDir(procdir)
	if err != nil {
		return 0, nil
	}
	// Only read the proc directory if absolutely necessary.
	if db.WillBePrinted(db.MSCHEDCLNT) {
		for _, st := range sts {
			b, err := mc.GetFile(filepath.Join(procdir, st.Name))
			if err != nil { // the proc may not exist anymore
				continue
			}
			p := proc.NewEmptyProc()
			p.Unmarshal(b)
			db.DPrintf(db.MSCHEDCLNT, "%s: %v", procdir, p.GetProgram())
		}
	}
	return len(sts), nil
}

func (mc *MSchedClnt) WarmProcd(kernelID string, pid sp.Tpid, realm sp.Trealm, prog string, path []string, ptype proc.Ttype) error {
	rpcc, err := mc.GetRPCClnt(kernelID)
	if err != nil {
		return err
	}
	req := &proto.WarmCacheBinReq{
		PidStr:    pid.String(),
		RealmStr:  realm.String(),
		Program:   prog,
		SigmaPath: path,
		ProcType:  int32(ptype),
	}
	res := &proto.WarmCacheBinRep{}
	if err := rpcc.RPC("MSched.WarmProcd", req, res); err != nil {
		return err
	}
	if !res.OK {
		db.DPrintf(db.ERROR, "WarmProcd failed realm %v prog %v tag %v", prog, prog, path)
		return fmt.Errorf("WarmProcd failed: realm %v prog %v tag %v", prog, prog, path)
	}
	return nil
}

// memAccountedFor should be false, unless this is a BE proc which the procqsrv
// is pushing to msched (the msched asked for it, and accounted for its
// memory).
func (mc *MSchedClnt) ForceRun(kernelID string, memAccountedFor bool, p *proc.Proc) error {
	start := time.Now()
	rpcc, err := mc.GetRPCClnt(kernelID)
	if err != nil {
		return err
	}
	perf.LogSpawnLatency("GetMSchedClnt", p.GetPid(), p.GetSpawnTime(), start)
	req := &proto.ForceRunReq{
		ProcProto:       p.GetProto(),
		MemAccountedFor: memAccountedFor,
	}
	res := &proto.ForceRunRep{}
	if err := rpcc.RPC("MSched.ForceRun", req, res); err != nil {
		return err
	}
	return nil
}

func (mc *MSchedClnt) Wait(method Tmethod, mschedID string, seqno *proc.ProcSeqno, pid sp.Tpid) (*proc.Status, error) {
	// RPC a msched to wait.
	rpcc, err := mc.GetRPCClnt(mschedID)
	if err != nil {
		return nil, err
	}
	req := &proto.WaitReq{
		PidStr:    pid.String(),
		ProcSeqno: seqno,
	}
	res := &proto.WaitRep{}
	if err := rpcc.RPC("MSched.Wait"+method.String(), req, res); err != nil {
		return nil, err
	}
	return proc.NewStatusFromBytes(res.Status), nil
}

func (mc *MSchedClnt) Notify(method Tmethod, kernelID string, pid sp.Tpid, status *proc.Status) error {
	start := time.Now()
	// Get the RPC client for the local msched
	rpcc, err := mc.GetRPCClnt(kernelID)
	if err != nil {
		return err
	}
	if method == START {
		perf.LogSpawnLatency("MSchedClnt.Notify started GetClnt", pid, perf.TIME_NOT_SET, start)
	}
	var b []byte
	if status != nil {
		b = status.Marshal()
	}
	req := &proto.NotifyReq{
		PidStr: pid.String(),
		Status: b,
	}
	res := &proto.NotifyRep{}
	start = time.Now()
	if err := rpcc.RPC("MSched."+method.Verb(), req, res); err != nil {
		return err
	}
	if method == START {
		perf.LogSpawnLatency("MSchedClnt.Notify started RPC", pid, perf.TIME_NOT_SET, start)
	}
	return nil
}

func (mc *MSchedClnt) GetAllRunningProcs() (map[sp.Trealm][]*proc.Proc, error) {
	mscheds, err := mc.GetMScheds()
	if err != nil {
		db.DPrintf(db.ERROR, "Err GetMScheds: %v", err)
		return nil, err
	}
	db.DPrintf(db.MSCHEDCLNT, "Get running procs on KIDs %v", mscheds)
	db.DPrintf(db.ALWAYS, "Get running procs on KIDs %v", mscheds)
	procs := make(map[sp.Trealm][]*proc.Proc, 0)
	for _, kid := range mscheds {
		running, err := mc.GetRunningProcs(kid)
		if err != nil {
			db.DPrintf(db.ERROR, "Err GetAllRunningProcs GetRunningProcs: %v", err)
			return nil, err
		}
		for r, ps := range running {
			procs[r] = append(procs[r], ps...)
		}
	}
	return procs, nil
}

func (mc *MSchedClnt) GetRunningProcs(kernelID string) (map[sp.Trealm][]*proc.Proc, error) {
	procs := make(map[sp.Trealm][]*proc.Proc, 0)
	req := &proto.GetRunningProcsReq{}
	res := &proto.GetRunningProcsRep{}
	rpcc, err := mc.GetRPCClnt(kernelID)
	if err != nil {
		db.DPrintf(db.ERROR, "Can't get clnt: %v", err)
		return nil, err
	}
	if err := rpcc.RPC("MSched.GetRunningProcs", req, res); err != nil {
		db.DPrintf(db.ERROR, "Err GetRunningProcs: %v", err)
		return nil, err
	}
	for _, pp := range res.ProcProtos {
		p := proc.NewProcFromProto(pp)
		r := p.GetRealm()
		if _, ok := procs[r]; !ok {
			procs[r] = make([]*proc.Proc, 0, 1)
		}
		procs[r] = append(procs[r], p)
	}
	return procs, nil
}

func (mc *MSchedClnt) SampleRunningProcs(nsample int) (map[sp.Trealm][]*proc.Proc, error) {
	// map of realm -> proc
	procs := make(map[sp.Trealm][]*proc.Proc, 0)
	sampled := make(map[string]bool)
	for i := 0; i < nsample; i++ {
		kernelID, err := mc.rpcdc.WaitTimedRandomEntry()
		if err != nil {
			db.DPrintf(db.ERROR, "Can't get random srv: %v", err)
			return nil, err
		}
		// Don't double-sample
		if sampled[kernelID] {
			continue
		}
		sampled[kernelID] = true
		running, err := mc.GetRunningProcs(kernelID)
		if err != nil {
			db.DPrintf(db.ERROR, "Err SampleRunningProcs GetRunningProcs: %v", err)
			return nil, err
		}
		for r, ps := range running {
			procs[r] = append(procs[r], ps...)
		}
	}
	return procs, nil
}

func (mc *MSchedClnt) MSchedStats() (int, []map[string]*proto.RealmStats, error) {
	sds, err := mc.rpcdc.GetEntries()
	if err != nil {
		return 0, nil, err
	}
	sdstats := make([]map[string]*proto.RealmStats, 0, len(sds))
	for _, sd := range sds {
		req := &proto.GetMSchedStatsReq{}
		res := &proto.GetMSchedStatsRep{}
		rpcc, err := mc.GetRPCClnt(sd)
		if err != nil {
			return 0, nil, err
		}
		if err := rpcc.RPC("MSched.GetMSchedStats", req, res); err != nil {
			return 0, nil, err
		}
		sdstats = append(sdstats, res.MSchedStats)
	}
	return len(sds), sdstats, err
}

func (mc *MSchedClnt) Done() {
	atomic.StoreInt32(&mc.done, 1)
}

func (mc *MSchedClnt) MonitorMSchedStats(realm sp.Trealm, period time.Duration) {
	go func() {
		for atomic.LoadInt32(&mc.done) == 0 {
			n, stats, err := mc.MSchedStats()
			if err != nil && atomic.LoadInt32(&mc.done) == 0 {
				db.DPrintf(db.ALWAYS, "MSchedStats err %v", err)
				return
			}
			r := realm.String()
			statsStr := ""
			for _, st := range stats {
				if rs, ok := st[r]; ok {
					statsStr += fmt.Sprintf(" [ r:%v t:%v ]", rs.Running, rs.TotalRan)
				}
			}
			db.DPrintf(db.ALWAYS, "[%v] msched stats = %d%v", r, n, statsStr)
			time.Sleep(period)
		}
	}()
}

func (mc *MSchedClnt) GetCPUUtil(realm sp.Trealm) (float64, error) {
	// Total CPU utilization by this sceddclnt's realm.
	var total float64 = 0
	// Get list of mscheds
	sds, err := mc.rpcdc.GetEntries()
	if err != nil {
		db.DPrintf(db.MSCHEDCLNT_ERR, "Error getMScheds: %v", err)
		return 0, err
	}
	for _, sd := range sds {
		// Get the CPU shares on this msched.
		req := &proto.GetCPUUtilReq{RealmStr: realm.String()}
		res := &proto.GetCPUUtilRep{}
		sclnt, err := mc.GetRPCClnt(sd)
		if err != nil {
			db.DPrintf(db.MSCHEDCLNT_ERR, "Error GetCPUUtil GetMSchedClnt: %v", err)
			return 0, err
		}
		err = sclnt.RPC("MSched.GetCPUUtil", req, res)
		if err != nil {
			db.DPrintf(db.MSCHEDCLNT_ERR, "Error GetCPUUtil: %v", err)
			return 0, err
		}
		db.DPrintf(db.CPU_UTIL, "MSched %v CPU util %v", sd, res.Util)
		total += res.Util
	}
	return total, nil
}

func (mc *MSchedClnt) StopWatching() {
	mc.rpcdc.StopWatching()
}

// Get the RPC client for my kernel's msched
func (mc *MSchedClnt) getRPCClntMyMSched() (*rpcclnt.RPCClnt, error) {
	mc.Lock()
	defer mc.Unlock()

	if mc.rpcc == nil {
		start := time.Now()
		pn := filepath.Join(sp.MSCHED, mc.kernelID)
		rpcc, err := sprpcclnt.NewRPCClnt(mc.FsLib, pn)
		if err != nil {
			return nil, err
		}
		perf.LogSpawnLatency("MSchedClnt.getRPCClntMyMSched", mc.ProcEnv().GetPID(), mc.ProcEnv().GetSpawnTime(), start)
		mc.rpcc = rpcc
	}
	return mc.rpcc, nil
}

func (mc *MSchedClnt) GetRPCClnt(kernelID string) (*rpcclnt.RPCClnt, error) {
	if kernelID == mc.kernelID {
		return mc.getRPCClntMyMSched()
	}
	rpcc, err := mc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return nil, err
	}
	return rpcc, nil
}
