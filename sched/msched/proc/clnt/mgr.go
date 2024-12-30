package clnt

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	db "sigmaos/debug"
	kernelclnt "sigmaos/kernel/clnt"
	"sigmaos/proc"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type startProcdFn func() (sp.Tpid, *ProcClnt)

type ProcdMgr struct {
	mu            sync.Mutex
	fsl           *fslib.FsLib
	kernelId      string
	pool          *pool
	kclnt         *kernelclnt.KernelClnt
	upcs          map[sp.Trealm]map[proc.Ttype]*ProcClnt // We use a separate procd for each type of proc (BE or LC) to simplify cgroup management.
	realms        map[sp.Trealm]bool
	beProcds      []*ProcClnt
	sharesAlloced Tshare
}

func NewProcdMgr(fsl *fslib.FsLib, kernelId string) *ProcdMgr {
	pdm := &ProcdMgr{
		fsl:           fsl,
		kernelId:      kernelId,
		upcs:          make(map[sp.Trealm]map[proc.Ttype]*ProcClnt),
		beProcds:      make([]*ProcClnt, 0),
		sharesAlloced: 0,
	}
	pdm.pool = newPool(pdm.startProcd)
	go func() {
		// In the boot sequence, msched and other servers are started before the
		// kernel server registers itself in the namespace. However, we need the
		// kernel server to have started in order to boot more procds. Thus, we
		// wait for the kernel to start up and advertise itself in a separate
		// goroutine, and then fill the procd pool
		for {
			err := pdm.fsl.WaitCreate(filepath.Join(sp.BOOT, pdm.kernelId))
			// Retry if unreachable
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				db.DPrintf(db.PROCDMGR, "Boot dir unreachable")
				continue
			}
			if err != nil {
				db.DFatalf("Error WaitCreate for kernel: %v", err)
			}
			break
		}
		kclnt, err := kernelclnt.NewKernelClnt(pdm.fsl, filepath.Join(sp.BOOT, pdm.kernelId)+"/")
		if err != nil {
			db.DFatalf("Err ProcdMgr Can't make kernelclnt: %v", err)
		}
		if err := pdm.fsl.MkDir(filepath.Join(sp.MSCHED, pdm.kernelId, sp.PROCDREL), 0777); err != nil {
			db.DFatalf("Err mkdir for procds: %v", err)
		}
		pdm.kclnt = kclnt
		// Must have kclnt set in order to fill.
		pdm.pool.fill()
	}()
	return pdm
}

func (pdm *ProcdMgr) WarmStartProcd(realm sp.Trealm, ptype proc.Ttype) error {
	start := time.Now()
	defer func() {
		db.DPrintf(db.REALM_GROW_LAT, "[%v] WarmStartProcd latency: %v", realm, time.Since(start))
	}()

	pdm.mu.Lock()
	defer pdm.mu.Unlock()

	_, err := pdm.getClntOrStartProcd(realm, ptype)
	return err
}

// Return the CPU shares allocated to each realm.
func (pdm *ProcdMgr) GetCPUShares() map[sp.Trealm]Tshare {
	pdm.mu.Lock()
	defer pdm.mu.Unlock()

	smap := make(map[sp.Trealm]Tshare, len(pdm.upcs))
	for r, pdcm := range pdm.upcs {
		smap[r] = 0
		for _, rpcc := range pdcm {
			smap[r] += rpcc.share
		}
	}
	return smap
}

// Return the CPU utilization of a realm.
func (pdm *ProcdMgr) GetCPUUtil(realm sp.Trealm) float64 {
	// Get the rpcclnts relevant to this realm
	pdm.mu.Lock()
	var pdcs []*ProcClnt
	if m, ok := pdm.upcs[realm]; ok {
		pdcs = make([]*ProcClnt, 0, len(m))
		for _, pdc := range m {
			pdcs = append(pdcs, pdc)
		}
	} else {
		db.DPrintf(db.PROCDMGR, "No procs from realm %v", realm)
	}
	pdm.mu.Unlock()

	var total float64 = 0.0

	// Get CPU util for BE & LC procds, if there are any.
	for _, rpcc := range pdcs {
		util, err := pdm.kclnt.GetCPUUtil(rpcc.pid)
		if err != nil {
			db.DPrintf(db.ERROR, "Error GetCPUUtil: %v", err)
		}
		total += util
		db.DPrintf(db.PROCDMGR, "[%v] CPU util pid:%v util:%v", realm, rpcc.pid, util)
	}
	return total
}

func (pdm *ProcdMgr) startProcd() (sp.Tpid, *ProcClnt) {
	s := time.Now()
	pid, err := pdm.kclnt.Boot("procd", []string{pdm.kernelId}, []string{})
	db.DPrintf(db.SPAWN_LAT, "Boot procd latency: %v", time.Since(s))
	if err != nil {
		db.DFatalf("Error Boot Procd: %v", err)
	}
	pn := filepath.Join(sp.MSCHED, pdm.kernelId, sp.PROCDREL, pid.String())
	rc, err := sprpcclnt.NewRPCClnt(pdm.fsl, pn)
	if err != nil {
		db.DPrintf(db.ERROR, "Error Make RPCClnt Procd: %v", err)
	}
	c := NewProcClnt(pid, rc)
	return pid, c
}

func (pdm *ProcdMgr) getClntOrStartProcd(realm sp.Trealm, ptype proc.Ttype) (*ProcClnt, error) {
	pdcm, ok1 := pdm.upcs[realm]
	if !ok1 {
		pdcm = make(map[proc.Ttype]*ProcClnt)
		pdm.upcs[realm] = pdcm
	}
	rpcc, ok2 := pdcm[ptype]
	if !ok1 || !ok2 {
		start := time.Now()
		pid, clnt := pdm.pool.get()
		db.DPrintf(db.REALM_GROW_LAT, "[%v] ProcdMgr.pool.get latency: %v", realm, time.Since(start))
		db.DPrintf(db.PROCDMGR, "[realm:%v] get procd %v ptype %v", realm, pid, ptype)
		pdm.upcs[realm][ptype] = clnt
		rpcc = clnt
		if ptype == proc.T_BE {
			pdm.beProcds = append(pdm.beProcds, rpcc)
		}
	}
	return rpcc, nil
}

func (pdm *ProcdMgr) delProcClnt(realm sp.Trealm, ptype proc.Ttype) error {
	pdm.mu.Lock()
	defer pdm.mu.Unlock()

	pdcm, ok1 := pdm.upcs[realm]
	rpcc, ok2 := pdcm[ptype]
	if !ok1 || !ok2 {
		db.DFatalf("delProcClnt %v %v", realm, ptype)
	}
	delete(pdcm, ptype)
	if ptype == proc.T_BE {
		for i, r := range pdm.beProcds {
			if r == rpcc {
				pdm.beProcds = append(pdm.beProcds[0:i], pdm.beProcds[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (pdm *ProcdMgr) lookupClnt(realm sp.Trealm, ptype proc.Ttype) (*ProcClnt, error) {
	pdm.mu.Lock()
	defer pdm.mu.Unlock()

	// Try to either get the clnt for an existing procd, or start a new procd
	// for the realm & return its client
	return pdm.getClntOrStartProcd(realm, ptype)
}

func (pdm *ProcdMgr) RunUProc(uproc *proc.Proc) (uprocErr error, childErr error) {
	db.DPrintf(db.PROCDMGR, "[RunUProc %v] run uproc %v", uproc.GetRealm(), uproc)
	rpcc, err := pdm.lookupClnt(uproc.GetRealm(), uproc.GetType())
	if err != nil {
		return err, nil
	}
	// run and exit do resource accounting and share rebalancing for the
	// procds.
	if err := pdm.startBalanceShares(uproc); err != nil {
		pdm.delProcClnt(uproc.GetRealm(), uproc.GetType())
		db.DPrintf(db.PROCDMGR, "[RunUProc %v] delProcClnt %v", uproc.GetRealm(), uproc)
		return err, nil
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Balance Procd shares time since spawn %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))
	if err0, err1 := rpcc.RunProc(uproc); err0 != nil {
		pdm.delProcClnt(uproc.GetRealm(), uproc.GetType())
		return err0, err1
	} else {
		pdm.exitBalanceShares(uproc)
		return nil, err1
	}
}

func (pdm *ProcdMgr) WarmProcd(pid sp.Tpid, realm sp.Trealm, prog string, path []string, ptype proc.Ttype) (uprocErr error, childErr error) {
	db.DPrintf(db.PROCDMGR, "[WarmUproc %v] warm uproc %v", prog)
	rpcc, err := pdm.lookupClnt(realm, ptype)
	if err != nil {
		return err, nil
	}
	ep, err := pdm.fsl.GetNamedEndpointRealm(realm)
	if err != nil {
		db.DPrintf(db.ERROR, "Error get realm named EP in WarmProcd: %v", err)
		return err, nil
	}
	return rpcc.WarmProcd(pid, realm, prog, pdm.fsl.ProcEnv().GetSecrets()["s3"], ep, path)
}

func (pdm *ProcdMgr) String() string {
	clnts := make([]*ProcClnt, 0)
	for _, m := range pdm.upcs {
		for _, c := range m {
			clnts = append(clnts, c)
		}
	}
	return fmt.Sprintf("&{ clnts:%v}", clnts)
}
