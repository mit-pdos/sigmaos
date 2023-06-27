package scheddclnt

import (
	"fmt"
	"path"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/schedd/proto"
	sp "sigmaos/sigmap"
	"sigmaos/uprocclnt"
)

type ScheddClnt struct {
	done  int32
	realm sp.Trealm
	*fslib.FsLib
	schedds map[string]*protdevclnt.ProtDevClnt
}

type Tload [2]int

func (t Tload) String() string {
	return fmt.Sprintf("{r %d q %d}", t[0], t[1])
}

func MakeScheddClnt(fsl *fslib.FsLib, realm sp.Trealm) *ScheddClnt {
	return &ScheddClnt{0, realm, fsl, make(map[string]*protdevclnt.ProtDevClnt)}
}

func (sdc *ScheddClnt) Nschedd() (int, error) {
	sds, err := sdc.getSchedds()
	if err != nil {
		return 0, err
	}
	return len(sds), nil
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
			p := proc.MakeEmptyProc()
			p.Unmarshal(b)
			db.DPrintf(db.SCHEDDCLNT, "%s: %v\n", procdir, p.Program)
		}
	}
	return len(sts), nil
}

func (sdc *ScheddClnt) ScheddLoad() (int, []Tload, error) {
	sds, err := sdc.getSchedds()
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
		qproc, err := sdc.Nprocs(path.Join(sdpath, sp.QUEUE))
		if err != nil {
			return r, nil, err
		}
		sdloads = append(sdloads, Tload{nproc, qproc})
	}
	return r, sdloads, err
}

func (sdc *ScheddClnt) MonitorSchedds() {
	if true {
		return
	}
	var realmstr string
	if sdc.realm != "" {
		realmstr = "[" + sdc.realm.String() + "] "
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

// Return the CPU shares assigned to this realm, and the total CPU shares in
// the cluster
func (sdc *ScheddClnt) GetCPUShares() (rshare uprocclnt.Tshare, total uprocclnt.Tshare) {
	// Total CPU shares in the system
	total = 0
	// Target realm's share
	rshare = 0
	// Get list of schedds
	sds, err := sdc.getSchedds()
	if err != nil {
		db.DFatalf("Error getSchedds: %v", err)
	}
	for _, sd := range sds {
		// Get the CPU shares on this schedd.
		req := &proto.GetCPUSharesRequest{}
		res := &proto.GetCPUSharesResponse{}
		sclnt, err := sdc.getScheddClnt(sd)
		if err != nil {
			db.DFatalf("Error GetCPUShares RPC [schedd:%v]: %v", sd, err)
		}
		err = sclnt.RPC("Schedd.GetCPUShares", req, res)
		if err != nil {
			db.DFatalf("Error GetCPUShares RPC [schedd:%v]: %v", sd, err)
		}
		rshare += uprocclnt.Tshare(res.Shares[sdc.realm.String()])
		for _, share := range res.Shares {
			total += uprocclnt.Tshare(share)
		}
	}
	return rshare, total
}

func (sdc *ScheddClnt) GetCPUUtil() (float64, error) {
	// Total CPU utilization by this sceddclnt's realm.
	var total float64 = 0
	// Get list of schedds
	sds, err := sdc.getSchedds()
	if err != nil {
		db.DPrintf(db.SCHEDDCLNT_ERR, "Error getSchedds: %v", err)
		return 0, err
	}
	for _, sd := range sds {
		// Get the CPU shares on this schedd.
		req := &proto.GetCPUUtilRequest{RealmStr: sdc.realm.String()}
		res := &proto.GetCPUUtilResponse{}
		sclnt, err := sdc.getScheddClnt(sd)
		if err != nil {
			db.DPrintf(db.SCHEDDCLNT_ERR, "Error GetCPUUtil getScheddClnt: %v", err)
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
	sds, err := sdc.getSchedds()
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
			p := proc.MakeEmptyProc()
			p.UnmarshalJson(b)
			procs[sd] = append(procs[sd], p)
		}
	}
	return procs
}

func (sdc *ScheddClnt) Done() {
	atomic.StoreInt32(&sdc.done, 1)
}

func (sdc *ScheddClnt) getScheddClnt(kernelId string) (*protdevclnt.ProtDevClnt, error) {
	var pdc *protdevclnt.ProtDevClnt
	var ok bool
	if pdc, ok = sdc.schedds[kernelId]; !ok {
		var err error
		pdc, err = protdevclnt.MkProtDevClnt([]*fslib.FsLib{sdc.FsLib}, path.Join(sp.SCHEDD, kernelId))
		if err != nil {
			db.DPrintf(db.SCHEDDCLNT_ERR, "Error mkProtDevClnt[schedd:%v]: %v", kernelId, err)
			return nil, err
		}
		sdc.schedds[kernelId] = pdc
	}
	return pdc, nil
}

func (sdc *ScheddClnt) getSchedds() ([]string, error) {
	sts, err := sdc.GetDir(sp.SCHEDD)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
}
