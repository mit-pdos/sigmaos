package scheddclnt

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rpcdirclnt"
	"sigmaos/schedsrv/proto"
	sp "sigmaos/sigmap"
)

type ScheddClnt struct {
	*fslib.FsLib
	rpcdc *rpcdirclnt.RPCDirClnt
	done  int32
}

func NewScheddClnt(fsl *fslib.FsLib) *ScheddClnt {
	return &ScheddClnt{
		FsLib: fsl,
		rpcdc: rpcdirclnt.NewRPCDirClnt(fsl, sp.SCHEDD, db.SCHEDDCLNT, db.SCHEDDCLNT_ERR),
	}
}

func (sdc *ScheddClnt) Nschedd() (int, error) {
	return sdc.rpcdc.Nentry()
}

func (sdc *ScheddClnt) GetSchedds() ([]string, error) {
	return sdc.rpcdc.WaitTimedGetEntriesN(1)
}

func (sdc *ScheddClnt) UnregisterSrv(scheddID string) {
	sdc.rpcdc.InvalidateEntry(scheddID)
}

func (sdc *ScheddClnt) Nprocs(procdir string) (int, error) {
	sts, err := sdc.GetDir(procdir)
	if err != nil {
		return 0, nil
	}
	// Only read the proc directory if absolutely necessary.
	if db.WillBePrinted(db.SCHEDDCLNT) {
		for _, st := range sts {
			b, err := sdc.GetFile(filepath.Join(procdir, st.Name))
			if err != nil { // the proc may not exist anymore
				continue
			}
			p := proc.NewEmptyProc()
			p.Unmarshal(b)
			db.DPrintf(db.SCHEDDCLNT, "%s: %v", procdir, p.GetProgram())
		}
	}
	return len(sts), nil
}

func (sdc *ScheddClnt) WarmUprocd(kernelID string, pid sp.Tpid, realm sp.Trealm, prog string, path []string, ptype proc.Ttype) error {
	rpcc, err := sdc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	req := &proto.WarmCacheBinRequest{
		PidStr:    pid.String(),
		RealmStr:  realm.String(),
		Program:   prog,
		SigmaPath: path,
		ProcType:  int32(ptype),
	}
	res := &proto.WarmCacheBinResponse{}
	if err := rpcc.RPC("Schedd.WarmUprocd", req, res); err != nil {
		return err
	}
	if !res.OK {
		db.DPrintf(db.ERROR, "WarmUprocd failed realm %v prog %v tag %v", prog, prog, path)
		return fmt.Errorf("WarmUprocd failed: realm %v prog %v tag %v", prog, prog, path)
	}
	return nil
}

// memAccountedFor should be false, unless this is a BE proc which the procqsrv
// is pushing to schedd (the schedd asked for it, and accounted for its
// memory).
func (sdc *ScheddClnt) ForceRun(kernelID string, memAccountedFor bool, p *proc.Proc) error {
	start := time.Now()
	rpcc, err := sdc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] GetScheddClnt time %v", p.GetPid(), time.Since(start))
	req := &proto.ForceRunRequest{
		ProcProto:       p.GetProto(),
		MemAccountedFor: memAccountedFor,
	}
	res := &proto.ForceRunResponse{}
	if err := rpcc.RPC("Schedd.ForceRun", req, res); err != nil {
		return err
	}
	return nil
}

func (sdc *ScheddClnt) Wait(method Tmethod, scheddID string, seqno *proc.ProcSeqno, pid sp.Tpid) (*proc.Status, error) {
	// RPC a schedd to wait.
	rpcc, err := sdc.rpcdc.GetClnt(scheddID)
	if err != nil {
		return nil, err
	}
	req := &proto.WaitRequest{
		PidStr:    pid.String(),
		ProcSeqno: seqno,
	}
	res := &proto.WaitResponse{}
	if err := rpcc.RPC("Schedd.Wait"+method.String(), req, res); err != nil {
		return nil, err
	}
	return proc.NewStatusFromBytes(res.Status), nil
}

func (sdc *ScheddClnt) Notify(method Tmethod, kernelID string, pid sp.Tpid, status *proc.Status) error {
	start := time.Now()
	// Get the RPC client for the local schedd
	rpcc, err := sdc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	if method == START {
		db.DPrintf(db.SPAWN_LAT, "[%v] scheddclnt.Notify Started rpcdc.GetClnt latency: %v", pid, time.Since(start))
	}
	var b []byte
	if status != nil {
		b = status.Marshal()
	}
	req := &proto.NotifyRequest{
		PidStr: pid.String(),
		Status: b,
	}
	res := &proto.NotifyResponse{}
	start = time.Now()
	if err := rpcc.RPC("Schedd."+method.Verb(), req, res); err != nil {
		return err
	}
	if method == START {
		db.DPrintf(db.SPAWN_LAT, "[%v] Notify RPC latency: %v", pid, time.Since(start))
	}
	return nil
}

func (sdc *ScheddClnt) Checkpoint(kernelID string, pid sp.Tpid, r sp.Trealm, pn string) error {
	db.DPrintf(db.ALWAYS, "Checkpoint kernelId %v pid %v", kernelID, pid)
	rpcc, err := sdc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	req := &proto.CheckpointProcRequest{
		PidStr:   pid.String(),
		RealmStr: r.String(),
		PathName: pn,
	}
	res := &proto.CheckpointProcResponse{}
	if err := rpcc.RPC("Schedd.CheckpointProc", req, res); err != nil {
		return err
	}
	return nil
}

func (sdc *ScheddClnt) GetRunningProcs(nsample int) (map[sp.Trealm][]*proc.Proc, error) {
	// map of realm -> proc
	procs := make(map[sp.Trealm][]*proc.Proc, 0)
	sampled := make(map[string]bool)
	for i := 0; i < nsample; i++ {
		kernelID, err := sdc.rpcdc.WaitTimedRandomEntry()
		if err != nil {
			db.DPrintf(db.ERROR, "Can't get random srv: %v", err)
			return nil, err
		}
		// Don't double-sample
		if sampled[kernelID] {
			continue
		}
		sampled[kernelID] = true
		req := &proto.GetRunningProcsRequest{}
		res := &proto.GetRunningProcsResponse{}
		rpcc, err := sdc.rpcdc.GetClnt(kernelID)
		if err != nil {
			db.DPrintf(db.ERROR, "Can't get clnt: %v", err)
			return nil, err
		}
		if err := rpcc.RPC("Schedd.GetRunningProcs", req, res); err != nil {
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

	}
	return procs, nil
}

func (sdc *ScheddClnt) ScheddStats() (int, []map[string]*proto.RealmStats, error) {
	sds, err := sdc.rpcdc.GetEntries()
	if err != nil {
		return 0, nil, err
	}
	sdstats := make([]map[string]*proto.RealmStats, 0, len(sds))
	for _, sd := range sds {
		req := &proto.GetScheddStatsRequest{}
		res := &proto.GetScheddStatsResponse{}
		rpcc, err := sdc.rpcdc.GetClnt(sd)
		if err != nil {
			return 0, nil, err
		}
		if err := rpcc.RPC("Schedd.GetScheddStats", req, res); err != nil {
			return 0, nil, err
		}
		sdstats = append(sdstats, res.ScheddStats)
	}
	return len(sds), sdstats, err
}

func (sdc *ScheddClnt) Done() {
	atomic.StoreInt32(&sdc.done, 1)
}

func (sdc *ScheddClnt) MonitorScheddStats(realm sp.Trealm, period time.Duration) {
	go func() {
		for atomic.LoadInt32(&sdc.done) == 0 {
			n, stats, err := sdc.ScheddStats()
			if err != nil && atomic.LoadInt32(&sdc.done) == 0 {
				db.DPrintf(db.ALWAYS, "ScheddStats err %v", err)
				return
			}
			r := realm.String()
			statsStr := ""
			for _, st := range stats {
				if rs, ok := st[r]; ok {
					statsStr += fmt.Sprintf(" [ r:%v t:%v ]", rs.Running, rs.TotalRan)
				}
			}
			db.DPrintf(db.ALWAYS, "[%v] schedd stats = %d%v", r, n, statsStr)
			time.Sleep(period)
		}
	}()
}

func (sdc *ScheddClnt) GetCPUUtil(realm sp.Trealm) (float64, error) {
	// Total CPU utilization by this sceddclnt's realm.
	var total float64 = 0
	// Get list of schedds
	sds, err := sdc.rpcdc.GetEntries()
	if err != nil {
		db.DPrintf(db.SCHEDDCLNT_ERR, "Error getSchedds: %v", err)
		return 0, err
	}
	for _, sd := range sds {
		// Get the CPU shares on this schedd.
		req := &proto.GetCPUUtilRequest{RealmStr: realm.String()}
		res := &proto.GetCPUUtilResponse{}
		sclnt, err := sdc.rpcdc.GetClnt(sd)
		if err != nil {
			db.DPrintf(db.SCHEDDCLNT_ERR, "Error GetCPUUtil GetScheddClnt: %v", err)
			return 0, err
		}
		err = sclnt.RPC("Schedd.GetCPUUtil", req, res)
		if err != nil {
			db.DPrintf(db.SCHEDDCLNT_ERR, "Error GetCPUUtil: %v", err)
			return 0, err
		}
		db.DPrintf(db.CPU_UTIL, "Schedd %v CPU util %v", sd, res.Util)
		total += res.Util
	}
	return total, nil
}

func (sdc *ScheddClnt) StopWatching() {
	sdc.rpcdc.StopWatching()
}
