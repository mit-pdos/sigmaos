package uprocclnt

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type startUprocdFn func() (sp.Tpid, *UprocdClnt)

type UprocdMgr struct {
	mu            sync.Mutex
	fsl           *fslib.FsLib
	kernelId      string
	pool          *pool
	kclnt         *kernelclnt.KernelClnt
	upcs          map[sp.Trealm]map[proc.Ttype]*UprocdClnt // We use a separate uprocd for each type of proc (BE or LC) to simplify cgroup management.
	realms        map[sp.Trealm]bool
	beUprocds     []*UprocdClnt
	sharesAlloced Tshare
}

func NewUprocdMgr(fsl *fslib.FsLib, kernelId string) *UprocdMgr {
	updm := &UprocdMgr{
		fsl:           fsl,
		kernelId:      kernelId,
		upcs:          make(map[sp.Trealm]map[proc.Ttype]*UprocdClnt),
		beUprocds:     make([]*UprocdClnt, 0),
		sharesAlloced: 0,
	}
	updm.pool = newPool(updm.startUprocd)
	go func() {
		// In the boot sequence, schedd and other servers are started before the
		// kernel server registers itself in the namespace. However, we need the
		// kernel server to have started in order to boot more uprocds. Thus, we
		// wait for the kernel to start up and advertise itself in a separate
		// goroutine, and then fill the uprocd pool
		for {
			err := updm.fsl.WaitCreate(filepath.Join(sp.BOOT, updm.kernelId))
			// Retry if unreachable
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.UPROCDMGR, "Boot dir unreachable")
				continue
			}
			if err != nil {
				db.DFatalf("Error WaitCreate for kernel: %v", err)
			}
			break
		}
		kclnt, err := kernelclnt.NewKernelClnt(updm.fsl, filepath.Join(sp.BOOT, updm.kernelId)+"/")
		if err != nil {
			db.DFatalf("Err UprocdMgr Can't make kernelclnt: %v", err)
		}
		if err := updm.fsl.MkDir(filepath.Join(sp.MSCHED, updm.kernelId, sp.UPROCDREL), 0777); err != nil {
			db.DFatalf("Err mkdir for uprocds: %v", err)
		}
		updm.kclnt = kclnt
		// Must have kclnt set in order to fill.
		updm.pool.fill()
	}()
	return updm
}

func (updm *UprocdMgr) WarmStartUprocd(realm sp.Trealm, ptype proc.Ttype) error {
	start := time.Now()
	defer func() {
		db.DPrintf(db.REALM_GROW_LAT, "[%v] WarmStartUprocd latency: %v", realm, time.Since(start))
	}()

	updm.mu.Lock()
	defer updm.mu.Unlock()

	_, err := updm.getClntOrStartUprocd(realm, ptype)
	return err
}

// Return the CPU shares allocated to each realm.
func (updm *UprocdMgr) GetCPUShares() map[sp.Trealm]Tshare {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	smap := make(map[sp.Trealm]Tshare, len(updm.upcs))
	for r, pdcm := range updm.upcs {
		smap[r] = 0
		for _, rpcc := range pdcm {
			smap[r] += rpcc.share
		}
	}
	return smap
}

// Return the CPU utilization of a realm.
func (updm *UprocdMgr) GetCPUUtil(realm sp.Trealm) float64 {
	// Get the rpcclnts relevant to this realm
	updm.mu.Lock()
	var pdcs []*UprocdClnt
	if m, ok := updm.upcs[realm]; ok {
		pdcs = make([]*UprocdClnt, 0, len(m))
		for _, pdc := range m {
			pdcs = append(pdcs, pdc)
		}
	} else {
		db.DPrintf(db.UPROCDMGR, "No procs from realm %v", realm)
	}
	updm.mu.Unlock()

	var total float64 = 0.0

	// Get CPU util for BE & LC uprocds, if there are any.
	for _, rpcc := range pdcs {
		util, err := updm.kclnt.GetCPUUtil(rpcc.pid)
		if err != nil {
			db.DPrintf(db.ERROR, "Error GetCPUUtil: %v", err)
		}
		total += util
		db.DPrintf(db.UPROCDMGR, "[%v] CPU util pid:%v util:%v", realm, rpcc.pid, util)
	}
	return total
}

func (updm *UprocdMgr) startUprocd() (sp.Tpid, *UprocdClnt) {
	s := time.Now()
	pid, err := updm.kclnt.Boot("uprocd", []string{updm.kernelId})
	db.DPrintf(db.SPAWN_LAT, "Boot uprocd latency: %v", time.Since(s))
	if err != nil {
		db.DFatalf("Error Boot Uprocd: %v", err)
	}
	pn := filepath.Join(sp.MSCHED, updm.kernelId, sp.UPROCDREL, pid.String())
	rc, err := sprpcclnt.NewRPCClnt(updm.fsl, pn)
	if err != nil {
		db.DPrintf(db.ERROR, "Error Make RPCClnt Uprocd: %v", err)
	}
	c := NewUprocdClnt(pid, rc)
	return pid, c
}

func (updm *UprocdMgr) getClntOrStartUprocd(realm sp.Trealm, ptype proc.Ttype) (*UprocdClnt, error) {
	pdcm, ok1 := updm.upcs[realm]
	if !ok1 {
		pdcm = make(map[proc.Ttype]*UprocdClnt)
		updm.upcs[realm] = pdcm
	}
	rpcc, ok2 := pdcm[ptype]
	if !ok1 || !ok2 {
		start := time.Now()
		pid, clnt := updm.pool.get()
		db.DPrintf(db.REALM_GROW_LAT, "[%v] UprocdMgr.pool.get latency: %v", realm, time.Since(start))
		db.DPrintf(db.UPROCDMGR, "[realm:%v] get uprocd %v ptype %v", realm, pid, ptype)
		updm.upcs[realm][ptype] = clnt
		rpcc = clnt
		if ptype == proc.T_BE {
			updm.beUprocds = append(updm.beUprocds, rpcc)
		}
	}
	return rpcc, nil
}

func (updm *UprocdMgr) lookupClnt(realm sp.Trealm, ptype proc.Ttype) (*UprocdClnt, error) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	// Try to either get the clnt for an existing uprocd, or start a new uprocd
	// for the realm & return its client
	return updm.getClntOrStartUprocd(realm, ptype)
}

func (updm *UprocdMgr) RunUProc(uproc *proc.Proc) (uprocErr error, childErr error) {
	db.DPrintf(db.UPROCDMGR, "[RunUProc %v] run uproc %v", uproc.GetRealm(), uproc)
	rpcc, err := updm.lookupClnt(uproc.GetRealm(), uproc.GetType())
	if err != nil {
		return err, nil
	}
	// run and exit do resource accounting and share rebalancing for the
	// uprocds.
	updm.startBalanceShares(uproc)
	db.DPrintf(db.SPAWN_LAT, "[%v] Balance Uprocd shares time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	defer updm.exitBalanceShares(uproc)
	return rpcc.RunProc(uproc)
}

func (updm *UprocdMgr) WarmProc(pid sp.Tpid, realm sp.Trealm, prog string, path []string, ptype proc.Ttype) (uprocErr error, childErr error) {
	db.DPrintf(db.UPROCDMGR, "[WarmUproc %v] warm uproc %v", prog)
	rpcc, err := updm.lookupClnt(realm, ptype)
	if err != nil {
		return err, nil
	}
	ep, err := updm.fsl.GetNamedEndpointRealm(realm)
	if err != nil {
		db.DPrintf(db.ERROR, "Error get realm named EP in WarmProc: %v", err)
		return err, nil
	}
	return rpcc.WarmProc(pid, realm, prog, updm.fsl.ProcEnv().GetSecrets()["s3"], ep, path)
}

func (updm *UprocdMgr) String() string {
	clnts := make([]*UprocdClnt, 0)
	for _, m := range updm.upcs {
		for _, c := range m {
			clnts = append(clnts, c)
		}
	}
	return fmt.Sprintf("&{ clnts:%v}", clnts)
}
