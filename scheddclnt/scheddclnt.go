package scheddclnt

import (
	"fmt"
	"path"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
	"sigmaos/schedd/proto"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/uprocclnt"
)

type ScheddClnt struct {
	done int32
	*fslib.FsLib
	sync.Mutex
	schedds         map[string]*rpcclnt.RPCClnt
	scheddKernelIds []string
	lastUpdate      time.Time
	burstOffset     int
}

type Tload [2]int

func (t Tload) String() string {
	return fmt.Sprintf("{r %d q %d}", t[0], t[1])
}

func MakeScheddClnt(fsl *fslib.FsLib) *ScheddClnt {
	return &ScheddClnt{
		FsLib:           fsl,
		schedds:         make(map[string]*rpcclnt.RPCClnt),
		scheddKernelIds: make([]string, 0),
	}
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

// Return the CPU shares assigned to this realm, and the total CPU shares in
// the cluster
func (sdc *ScheddClnt) GetCPUShares(realm sp.Trealm) (rshare uprocclnt.Tshare, total uprocclnt.Tshare) {
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
		sclnt, err := sdc.GetScheddClnt(sd)
		if err != nil {
			db.DFatalf("Error GetCPUShares RPC [schedd:%v]: %v", sd, err)
		}
		err = sclnt.RPC("Schedd.GetCPUShares", req, res)
		if err != nil {
			db.DFatalf("Error GetCPUShares RPC [schedd:%v]: %v", sd, err)
		}
		rshare += uprocclnt.Tshare(res.Shares[realm.String()])
		for _, share := range res.Shares {
			total += uprocclnt.Tshare(share)
		}
	}
	return rshare, total
}

func (sdc *ScheddClnt) GetCPUUtil(realm sp.Trealm) (float64, error) {
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
		req := &proto.GetCPUUtilRequest{RealmStr: realm.String()}
		res := &proto.GetCPUUtilResponse{}
		sclnt, err := sdc.GetScheddClnt(sd)
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

func (sdc *ScheddClnt) GetScheddClnt(kernelId string) (*rpcclnt.RPCClnt, error) {
	sdc.Lock()
	defer sdc.Unlock()

	var rpcc *rpcclnt.RPCClnt
	var ok bool
	if rpcc, ok = sdc.schedds[kernelId]; !ok {
		var err error
		rpcc, err = rpcclnt.MkRPCClnt([]*fslib.FsLib{sdc.FsLib}, path.Join(sp.SCHEDD, kernelId))
		if err != nil {
			db.DPrintf(db.SCHEDDCLNT_ERR, "Error mkRPCClnt[schedd:%v]: %v", kernelId, err)
			return nil, err
		}
		sdc.schedds[kernelId] = rpcc
	}
	return rpcc, nil
}

func (sdc *ScheddClnt) RegisterLocalClnt(pdc *rpcclnt.RPCClnt) error {
	sdc.Lock()
	defer sdc.Unlock()

	p, ok, err := sdc.ResolveUnion(path.Join(sp.SCHEDD, "~local"))
	if !ok || err != nil {
		// If ~local hasn't registered itself yet, this method should've bailed
		// out earlier.
		return fmt.Errorf("Couldn't register schedd ~local: %v, %v, %v", p, ok, err)
	}
	kernelId := path.Base(p)
	db.DPrintf(db.PROCCLNT, "Resolved ~local to %v", kernelId)
	sdc.schedds[kernelId] = pdc
	return nil
}

func (sdc *ScheddClnt) UnregisterClnt(kernelId string) {
	sdc.Lock()
	defer sdc.Unlock()
	delete(sdc.schedds, kernelId)
}

func (sdc *ScheddClnt) getSchedds() ([]string, error) {
	sts, err := sdc.GetDir(sp.SCHEDD)
	if err != nil {
		return nil, err
	}
	return sp.Names(sts), nil
}

// Update the list of active procds.
func (sdc *ScheddClnt) UpdateSchedds() {
	sdc.Lock()
	defer sdc.Unlock()

	// If we updated the list of active procds recently, return immediately. The
	// list will change at most as quickly as the realm resizes.
	if time.Since(sdc.lastUpdate) < sp.Conf.Realm.RESIZE_INTERVAL && len(sdc.scheddKernelIds) > 0 {
		db.DPrintf(db.PROCCLNT, "Update schedds too soon")
		return
	}
	sdc.lastUpdate = time.Now()
	// Read the procd union dir.
	schedds, _, err := sdc.ReadDir(sp.SCHEDD)
	if err != nil {
		db.DFatalf("Error ReadDir procd: %v", err)
	}
	db.DPrintf(db.PROCCLNT, "Got schedds %v", sp.Names(schedds))
	// Alloc enough space for the list of schedds.
	sdc.scheddKernelIds = make([]string, 0, len(schedds))
	for _, schedd := range schedds {
		sdc.scheddKernelIds = append(sdc.scheddKernelIds, schedd.Name)
	}
}

// Get the next procd to burst on.
func (sdc *ScheddClnt) NextSchedd(spread int) (string, error) {
	sdc.Lock()
	defer sdc.Unlock()

	if len(sdc.scheddKernelIds) == 0 {
		debug.PrintStack()
		return "", serr.MkErr(serr.TErrNotfound, "no schedds to spawn on")
	}

	sdip := sdc.scheddKernelIds[(sdc.burstOffset/spread)%len(sdc.scheddKernelIds)]
	sdc.burstOffset++
	return sdip, nil
}
