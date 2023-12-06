package scheddclnt

import (
	"fmt"
	"path"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
	"sigmaos/unionrpcclnt"
)

type ScheddClnt struct {
	*fslib.FsLib
	urpcc *unionrpcclnt.UnionRPCClnt
	done  int32
}

func NewScheddClnt(fsl *fslib.FsLib) *ScheddClnt {
	return &ScheddClnt{
		FsLib: fsl,
		urpcc: unionrpcclnt.NewUnionRPCClnt(fsl, sp.SCHEDD, db.SCHEDDCLNT, db.SCHEDDCLNT_ERR),
	}
}

func (sdc *ScheddClnt) Nschedd() (int, error) {
	sds, err := sdc.GetSchedds()
	if err != nil {
		return 0, err
	}
	return len(sds), nil
}

func (sdc *ScheddClnt) GetSchedds() ([]string, error) {
	return sdc.urpcc.GetSrvs()
}

func (sdc *ScheddClnt) NextSchedd() (string, error) {
	return sdc.urpcc.NextSrv()
}

func (sdc *ScheddClnt) UpdateSchedds() {
	sdc.urpcc.UpdateSrvs(false)
}

func (sdc *ScheddClnt) UnregisterSrv(scheddID string) {
	sdc.urpcc.UnregisterSrv(scheddID)
}

func (sdc *ScheddClnt) Nprocs(procdir string) (int, error) {
	sts, err := sdc.GetDir(procdir)
	if err != nil {
		return 0, nil
	}
	// Only read the proc directory if absolutely necessary.
	if db.WillBePrinted(db.SCHEDDCLNT) {
		for _, st := range sts {
			b, err := sdc.GetFile(path.Join(procdir, st.Name))
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

func (sdc *ScheddClnt) WarmCacheBin(kernelID string, realm sp.Trealm, prog, buildTag string, ptype proc.Ttype) error {
	rpcc, err := sdc.urpcc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	req := &proto.WarmCacheBinRequest{
		RealmStr: realm.String(),
		Program:  prog,
		BuildTag: buildTag,
		ProcType: int32(ptype),
	}
	res := &proto.WarmCacheBinResponse{}
	if err := rpcc.RPC("Schedd.WarmCacheBin", req, res); err != nil {
		return err
	}
	if !res.OK {
		db.DFatalf("Err couldn't warm cache bin: realm %v prog %v tag %v", prog, prog, buildTag)
	}
	return nil
}

// memAccountedFor should be false, unless this is a BE proc which the procqsrv
// is pushing to schedd (the schedd asked for it, and accounted for its
// memory).
func (sdc *ScheddClnt) ForceRun(kernelID string, memAccountedFor bool, p *proc.Proc) error {
	rpcc, err := sdc.urpcc.GetClnt(kernelID)
	if err != nil {
		return err
	}
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

func (sdc *ScheddClnt) Wait(method Tmethod, kernelID string, pid sp.Tpid) (*proc.Status, error) {
	// RPC a schedd to wait.
	rpcc, err := sdc.urpcc.GetClnt(kernelID)
	if err != nil {
		return nil, err
	}
	req := &proto.WaitRequest{
		PidStr: pid.String(),
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
	rpcc, err := sdc.urpcc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	if method == START {
		db.DPrintf(db.SPAWN_LAT, "[%v] scheddclnt.Notify Started urpcc.GetClnt latency: %v", pid, time.Since(start))
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

func (sdc *ScheddClnt) Checkpoint(kernelID string, p *proc.Proc) (string, int, error) {
	rpcc, err := sdc.urpcc.GetClnt(kernelID)
	if err != nil {
		return "", -1, err
	}
	req := &proto.CheckpointProcRequest{
		ProcProto: p.GetProto(),
	}
	res := &proto.CheckpointProcResponse{}
	if err := rpcc.RPC("Schedd.CheckpointProc", req, res); err != nil {
		return "there was an error", -1, err
	}
	return res.CheckpointLocation, int(res.OsPid), nil
}

func (sdc *ScheddClnt) ScheddStats() (int, []map[string]*proto.RealmStats, error) {
	sds, err := sdc.urpcc.GetSrvs()
	if err != nil {
		return 0, nil, err
	}
	sdstats := make([]map[string]*proto.RealmStats, 0, len(sds))
	for _, sd := range sds {
		req := &proto.GetScheddStatsRequest{}
		res := &proto.GetScheddStatsResponse{}
		rpcc, err := sdc.urpcc.GetClnt(sd)
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
	sds, err := sdc.urpcc.GetSrvs()
	if err != nil {
		db.DPrintf(db.SCHEDDCLNT_ERR, "Error getSchedds: %v", err)
		return 0, err
	}
	for _, sd := range sds {
		// Get the CPU shares on this schedd.
		req := &proto.GetCPUUtilRequest{RealmStr: realm.String()}
		res := &proto.GetCPUUtilResponse{}
		sclnt, err := sdc.urpcc.GetClnt(sd)
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
