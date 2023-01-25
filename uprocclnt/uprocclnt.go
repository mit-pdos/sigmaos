package uprocclnt

import (
	"path"
	"sync"

	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocdMgr struct {
	mu    sync.Mutex
	fsl   *fslib.FsLib
	kclnt *kernelclnt.KernelClnt
	pdcms map[sp.Trealm]map[proc.Ttype]*protdevclnt.ProtDevClnt // We use a separate uprocd for each type of proc (BE or LC) to simplify cgroup management.
}

func MakeUprocdMgr(fsl *fslib.FsLib) *UprocdMgr {
	updm := &UprocdMgr{fsl: fsl}
	updm.pdcms = make(map[sp.Trealm]map[proc.Ttype]*protdevclnt.ProtDevClnt)
	return updm
}

func (updm *UprocdMgr) startUprocd(realm sp.Trealm, ptype proc.Ttype) error {
	if updm.kclnt == nil {
		kclnt, err := kernelclnt.MakeKernelClnt(updm.fsl, sp.BOOT+"~local/")
		if err != nil {
			return err
		}
		updm.kclnt = kclnt
	}
	return updm.kclnt.Boot("uprocd", []string{realm.String(), ptype.String()})
}

// Fill out procd directory structure in which to register the uprocd.
func (updm *UprocdMgr) mkdirs(realm sp.Trealm, ptype proc.Ttype) error {
	d1 := path.Join(sp.PROCD, "~local", sp.UPROCDREL)
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

func (updm *UprocdMgr) lookupClnt(realm sp.Trealm, ptype proc.Ttype) (*protdevclnt.ProtDevClnt, error) {
	updm.mu.Lock()
	defer updm.mu.Unlock()
	pdcm, ok1 := updm.pdcms[realm]
	if !ok1 {
		pdcm = make(map[proc.Ttype]*protdevclnt.ProtDevClnt)
		updm.pdcms[realm] = pdcm
	}
	pdc, ok2 := pdcm[ptype]
	if !ok1 || !ok2 {
		if err := updm.mkdirs(realm, ptype); err != nil {
			return nil, err
		}
		if err := updm.startUprocd(realm, ptype); err != nil {
			return nil, err
		}
		pn := path.Join(sp.PROCD, "~local", sp.UPROCDREL, realm.String(), ptype.String())
		c, err := protdevclnt.MkProtDevClnt(updm.fsl, pn)
		if err != nil {
			return nil, err
		}
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
