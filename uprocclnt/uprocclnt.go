package uprocclnt

import (
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

const (
	CPU_SHARES_LC = 10000
	CPU_SHARES_BE = 1000
)

type UprocdMgr struct {
	mu    sync.Mutex
	fsl   *fslib.FsLib
	kclnt *kernelclnt.KernelClnt
	pdcms map[sp.Trealm]map[proc.Ttype]*UprocdClnt // We use a separate uprocd for each type of proc (BE or LC) to simplify cgroup management.
}

type UprocdClnt struct {
	pid proc.Tpid
	*protdevclnt.ProtDevClnt
}

func MakeUprocdClnt(pid proc.Tpid, pdc *protdevclnt.ProtDevClnt) *UprocdClnt {
	return &UprocdClnt{pid, pdc}
}

func MakeUprocdMgr(fsl *fslib.FsLib) *UprocdMgr {
	updm := &UprocdMgr{fsl: fsl}
	updm.pdcms = make(map[sp.Trealm]map[proc.Ttype]*UprocdClnt)
	return updm
}

func (updm *UprocdMgr) startUprocd(realm sp.Trealm, ptype proc.Ttype) (proc.Tpid, error) {
	if updm.kclnt == nil {
		kclnt, err := kernelclnt.MakeKernelClnt(updm.fsl, sp.BOOT+"~local/")
		if err != nil {
			return proc.Tpid(""), err
		}
		updm.kclnt = kclnt
	}
	pid, err := updm.kclnt.Boot("uprocd", []string{realm.String(), ptype.String()})
	if err != nil {
		return pid, err
	}
	// TODO: Set cpu shares differently for LC uprocds according to proc core requests.
	var cpuShares int64
	switch ptype {
	case proc.T_LC:
		cpuShares = CPU_SHARES_LC
	case proc.T_BE:
		cpuShares = CPU_SHARES_BE
	default:
		db.DFatalf("Unkown proc type: %v", ptype)
	}
	err = updm.kclnt.SetCPUShares(pid, cpuShares)
	if err != nil {
		return pid, err
	}
	return pid, nil
}

// Fill out procd directory structure in which to register the uprocd.
func (updm *UprocdMgr) mkdirs(realm sp.Trealm, ptype proc.Ttype) error {
	d1 := path.Join(sp.SCHEDD, "~local", sp.UPROCDREL)
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
		if err := updm.mkdirs(realm, ptype); err != nil {
			return nil, err
		}
		var pid proc.Tpid
		var err error
		if pid, err = updm.startUprocd(realm, ptype); err != nil {
			return nil, err
		}
		pn := path.Join(sp.SCHEDD, "~local", sp.UPROCDREL, realm.String(), ptype.String())
		rc, err := protdevclnt.MkProtDevClnt(updm.fsl, pn)
		if err != nil {
			return nil, err
		}
		c := MakeUprocdClnt(pid, rc)
		updm.pdcms[realm][ptype] = c
		pdc = c
	}
	return pdc, nil
}

func (updm *UprocdMgr) MakeUProc(uproc *proc.Proc) (uprocErr error, childErr error) {
	pdc, err := updm.lookupClnt(uproc.GetRealm(), uproc.GetType())
	if err != nil {
		return err, nil
	}
	req := &proto.RunRequest{
		ProcProto: uproc.GetProto(),
	}
	res := &proto.RunResult{}
	if err := pdc.RPC("UprocSrv.Run", req, res); err != nil && serr.IsErrUnreachable(err) {
		return err, nil
	} else {
		return nil, err
	}
}
