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

type Tload [1]int

func (t Tload) String() string {
	return fmt.Sprintf("{r %d}", t[0])
}

func NewScheddClnt(fsl *fslib.FsLib) *ScheddClnt {
	return &ScheddClnt{
		FsLib: fsl,
		urpcc: unionrpcclnt.NewUnionRPCClnt(fsl, sp.SCHEDD, db.SCHEDDCLNT, db.SCHEDDCLNT_ERR),
	}
}

func (sdc *ScheddClnt) Nschedd() (int, error) {
	sds, err := sdc.urpcc.GetSrvs()
	if err != nil {
		return 0, err
	}
	return len(sds), nil
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
			db.DPrintf(db.SCHEDDCLNT, "%s: %v\n", procdir, p.GetProgram())
		}
	}
	return len(sts), nil
}

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

func (sdc *ScheddClnt) ScheddLoad() (int, []Tload, error) {
	sds, err := sdc.urpcc.GetSrvs()
	if err != nil {
		return 0, nil, err
	}
	r := len(sds)
	sdloads := make([]Tload, 0, r)
	for _, sd := range sds {
		sdpath := path.Join(sp.SCHEDD, sd, sp.RUNNING)
		nproc, err := sdc.Nprocs(path.Join(sdpath, sp.RUNNING))
		if err != nil {
			return r, nil, err
		}
		sdloads = append(sdloads, Tload{nproc})
	}
	return r, sdloads, err
}

func (sdc *ScheddClnt) Done() {
	atomic.StoreInt32(&sdc.done, 1)
}

func (sdc *ScheddClnt) MonitorSchedds(realm sp.Trealm) {
	if true {
		return
	}
	var realmstr string
	if realm != "" {
		realmstr = "[" + realm.String() + "] "
	}
	go func() {
		for atomic.LoadInt32(&sdc.done) == 0 {
			n, load, err := sdc.ScheddLoad()
			if err != nil && atomic.LoadInt32(&sdc.done) == 0 {
				db.DFatalf("ScheddLoad err %v\n", err)
			}
			db.DPrintf(db.ALWAYS, "%vnschedd = %d %v\n", realmstr, n, load)
			// Sleep for 10 seconds, but do so in an interruptible way.
			for i := 0; i < 10 && atomic.LoadInt32(&sdc.done) == 0; i++ {
				time.Sleep(1 * time.Second)
			}
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

func (sdc *ScheddClnt) GetRunningProcs() map[string][]*proc.Proc {
	// Get list of schedds
	sds, err := sdc.urpcc.GetSrvs()
	if err != nil {
		db.DFatalf("Error getSchedds: %v", err)
	}
	procs := make(map[string][]*proc.Proc)
	for _, sd := range sds {
		sdrun := path.Join(sp.SCHEDD, sd, sp.RUNNING)
		sts, err := sdc.GetDir(sdrun)
		if err != nil {
			db.DFatalf("Error getdir: %v", err)
		}
		procs[sd] = []*proc.Proc{}
		for _, st := range sts {
			b, err := sdc.GetFile(path.Join(sdrun, st.Name))
			if err != nil {
				db.DFatalf("Error getfile: %v", err)
			}
			p := proc.NewEmptyProc()
			p.UnmarshalJson(b)
			procs[sd] = append(procs[sd], p)
		}
	}
	return procs
}
