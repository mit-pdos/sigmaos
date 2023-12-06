package uprocclnt

import (
	"fmt"
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
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
			_, err := updm.fsl.ReadDirWatch(sp.BOOT, func(sts []*sp.Stat) bool {
				for _, kid := range sp.Names(sts) {
					// If the kernel ID is in the boot directory, stop waiting
					if kid == updm.kernelId {
						return false
					}
				}
				return true
			})
			// Retry if unreachable
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.UPROCDMGR, "Boot dir unreachable")
				continue
			}
			if err != nil {
				db.DFatalf("Error ReadDirWatch for kernel: %v", err)
			}
			break
		}
		kclnt, err := kernelclnt.NewKernelClnt(updm.fsl, path.Join(sp.BOOT, updm.kernelId)+"/")
		if err != nil {
			db.DFatalf("Err UprocdMgr Can't make kernelclnt: %v", err)
		}
		if err := updm.fsl.MkDir(path.Join(sp.SCHEDD, updm.kernelId, sp.UPROCDREL), 0777); err != nil {
			db.DFatalf("Err mkdir for uprocds: %v", err)
		}
		updm.kclnt = kclnt
		// Must have kclnt set in order to fill.
		updm.pool.fill()
	}()
	return updm
}

// Fill out procd directory structure in which to register the uprocd.
func (updm *UprocdMgr) mkdirs(realm sp.Trealm, ptype proc.Ttype) error {
	d1 := path.Join(sp.SCHEDD, updm.kernelId, sp.UPROCDREL)
	// We may get ErrExists if the uprocd for a different type (within the same realm) has already started up.
	if err := updm.fsl.MkDir(d1, 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		return err
	}
	d2 := path.Join(d1, realm.String())
	if err := updm.fsl.MkDir(d2, 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		return err
	}
	d3 := path.Join(d2, ptype.String())
	if err := updm.fsl.MkDir(d3, 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		return err
	}
	return nil
}

func (updm *UprocdMgr) WarmStartUprocd(realm sp.Trealm, ptype proc.Ttype) error {
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
			db.DFatalf("Error GetCPUUtil: %v", err)
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
	pn := path.Join(sp.SCHEDD, updm.kernelId, sp.UPROCDREL, pid.String())
	rc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{updm.fsl}, pn)
	if err != nil {
		db.DFatalf("Error Make RPCClnt Uprocd: %v", err)
	}
	c := NewUprocdClnt(pid, rc)
	return pid, c
}

func (updm *UprocdMgr) CheckpointProc(uproc *proc.Proc) (chkptloc string, odPid int, err error) {
	db.DPrintf(db.UPROCDMGR, "[CheckpointProc %v] run uproc %v", uproc.GetRealm(), uproc)
	rpcc, err := updm.lookupClnt(uproc.GetRealm(), uproc.GetType())
	if err != nil {
		return "", -1, err
	}
	return rpcc.CheckpointProc(uproc)
}

func (updm *UprocdMgr) getClntOrStartUprocd(realm sp.Trealm, ptype proc.Ttype) (*UprocdClnt, error) {
	pdcm, ok1 := updm.upcs[realm]
	if !ok1 {
		pdcm = make(map[proc.Ttype]*UprocdClnt)
		updm.upcs[realm] = pdcm
	}
	rpcc, ok2 := pdcm[ptype]
	if !ok1 || !ok2 {
		pid, clnt := updm.pool.get()
		db.DPrintf(db.UPROCDMGR, "[realm:%v] get uprocd %v", realm, pid)
		updm.upcs[realm][ptype] = clnt
		rpcc = clnt
		if ptype == proc.T_BE {
			updm.beUprocds = append(updm.beUprocds, rpcc)
		}
		clnt.AssignToRealm(realm, ptype)
	}
	return rpcc, nil
}

func (updm *UprocdMgr) lookupClnt(realm sp.Trealm, ptype proc.Ttype) (*UprocdClnt, error) {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	return updm.getClntOrStartUprocd(realm, ptype)
}

func (updm *UprocdMgr) RunUProc(uproc *proc.Proc) (uprocErr error, childErr error) {
	db.DPrintf(db.UPROCDMGR, "[RunUProc %v] run uproc %v", uproc.GetRealm(), uproc)
	s := time.Now()
	rpcc, err := updm.lookupClnt(uproc.GetRealm(), uproc.GetType())
	if err != nil {
		return err, nil
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Lookup Uprocd clnt %v", updm.fsl.ProcEnv().GetPID(), time.Since(s))
	// run and exit do resource accounting and share rebalancing for the
	// uprocds.
	s = time.Now()
	updm.startBalanceShares(uproc)
	db.DPrintf(db.SPAWN_LAT, "[%v] Balance Uprocd shares %v", updm.fsl.ProcEnv().GetPID(), time.Since(s))
	uproc.FinalizeEnv(updm.fsl.ProcEnv().LocalIP, rpcc.pid)
	defer updm.exitBalanceShares(uproc)
	return rpcc.RunProc(uproc)
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
