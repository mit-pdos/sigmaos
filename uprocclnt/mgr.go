package uprocclnt

import (
	"fmt"
	"log"
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocdMgr struct {
	mu            sync.Mutex
	fsl           *fslib.FsLib
	scheddIp      string
	kernelId      string
	kclnt         *kernelclnt.KernelClnt
	pdcms         map[sp.Trealm]map[proc.Ttype]*UprocdClnt // We use a separate uprocd for each type of proc (BE or LC) to simplify cgroup management.
	beUprocds     []*UprocdClnt
	sharesAlloced Tshare
}

func MakeUprocdMgr(fsl *fslib.FsLib, kernelId, scheddIp string) *UprocdMgr {
	updm := &UprocdMgr{
		fsl:           fsl,
		kernelId:      kernelId,
		scheddIp:      scheddIp,
		pdcms:         make(map[sp.Trealm]map[proc.Ttype]*UprocdClnt),
		beUprocds:     make([]*UprocdClnt, 0),
		sharesAlloced: 0,
	}
	return updm
}

func (updm *UprocdMgr) startUprocd(realm sp.Trealm, ptype proc.Ttype) (proc.Tpid, error) {
	if err := updm.mkdirs(realm, ptype); err != nil {
		return proc.Tpid(""), err
	}
	if updm.kclnt == nil {
		pn := path.Join(sp.BOOT, updm.kernelId) + "/"
		kclnt, err := kernelclnt.MakeKernelClnt(updm.fsl, pn)
		if err != nil {
			return proc.Tpid(""), err
		}
		updm.kclnt = kclnt
	}
	pid, err := updm.kclnt.Boot("uprocd", []string{realm.String(), ptype.String(), updm.scheddIp})
	if err != nil {
		return pid, err
	}
	return pid, nil
}

// Fill out procd directory structure in which to register the uprocd.
func (updm *UprocdMgr) mkdirs(realm sp.Trealm, ptype proc.Ttype) error {
	d1 := path.Join(sp.SCHEDD, updm.scheddIp, sp.UPROCDREL)
	// We may get ErrExists if the uprocd for a different type (within the same realm) has already started up.
	if err := updm.fsl.MkDir(d1, 0777); err != nil && !serr.IsErrExists(err) {
		return err
	}
	d2 := path.Join(d1, realm.String())
	if err := updm.fsl.MkDir(d2, 0777); err != nil && !serr.IsErrExists(err) {
		return err
	}
	d3 := path.Join(d2, ptype.String())
	if err := updm.fsl.MkDir(d3, 0777); err != nil && !serr.IsErrExists(err) {
		return err
	}
	return nil
}

func (updm *UprocdMgr) lookupClnt(realm sp.Trealm, ptype proc.Ttype) (*UprocdClnt, error) {
	updm.mu.Lock()
	defer updm.mu.Unlock()
	pdcm, ok1 := updm.pdcms[realm]
	if !ok1 {
		pdcm = make(map[proc.Ttype]*UprocdClnt)
		updm.pdcms[realm] = pdcm
	}
	pdc, ok2 := pdcm[ptype]
	if !ok1 || !ok2 {
		var pid proc.Tpid
		var err error
		if pid, err = updm.startUprocd(realm, ptype); err != nil {
			return nil, err
		}
		pn := path.Join(sp.SCHEDD, updm.scheddIp, sp.UPROCDREL, realm.String(), ptype.String())
		rc, err := protdevclnt.MkProtDevClnt(updm.fsl, pn)
		if err != nil {
			return nil, err
		}
		c := MakeUprocdClnt(pid, rc, realm, ptype)
		updm.pdcms[realm][ptype] = c
		pdc = c
		if ptype == proc.T_BE {
			updm.beUprocds = append(updm.beUprocds, pdc)
		}
	}
	return pdc, nil
}

func (updm *UprocdMgr) RunUProc(uproc *proc.Proc) (uprocErr error, childErr error) {
	pdc, err := updm.lookupClnt(uproc.GetRealm(), uproc.GetType())
	if err != nil {
		return err, nil
	}
	db.DPrintf(db.UPROCDMGR, "[realm:%v pid:%v] run uproc %v", pdc.realm, pdc.pid, uproc)
	// run and exit do resource accounting and share rebalancing for the
	// uprocds.
	updm.startBalanceShares(uproc)
	defer updm.exitBalanceShares(uproc)
	req := &proto.RunRequest{
		ProcProto: uproc.GetProto(),
	}
	res := &proto.RunResult{}
	if err := pdc.RPC("UprocSrv.Run", req, res); err != nil && serr.IsErrUnreachable(err) {
		log.Printf("uprocsrv run err %v\n", err)
		return err, nil
	} else {
		return nil, err
	}
}

// Return the CPU shares allocated to each realm.
func (updm *UprocdMgr) GetCPUShares() map[sp.Trealm]Tshare {
	updm.mu.Lock()
	defer updm.mu.Unlock()

	smap := make(map[sp.Trealm]Tshare, len(updm.pdcms))
	for r, pdcm := range updm.pdcms {
		smap[r] = 0
		for _, pdc := range pdcm {
			smap[r] += pdc.share
		}
	}
	return smap
}

// Return the CPU utilization of a realm.
func (updm *UprocdMgr) GetCPUUtil(realm sp.Trealm) float64 {
	// Get the protdevclnts relevant to this realm
	updm.mu.Lock()
	var pdcs []*UprocdClnt
	if m, ok := updm.pdcms[realm]; ok {
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
	for _, pdc := range pdcs {
		util, err := updm.kclnt.GetCPUUtil(pdc.pid)
		if err != nil {
			db.DFatalf("Error GetCPUUtil: %v", err)
		}
		total += util
		db.DPrintf(db.UPROCDMGR, "[%v] CPU util pid:%v util:%v", realm, pdc.pid, util)
	}
	return total
}

func (updm *UprocdMgr) String() string {
	clnts := make([]*UprocdClnt, 0)
	for _, m := range updm.pdcms {
		for _, c := range m {
			clnts = append(clnts, c)
		}
	}
	return fmt.Sprintf("&{ clnts:%v}", clnts)
}
