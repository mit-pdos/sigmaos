package uprocclnt

import (
	"path"
	"sync"

	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocdMgr struct {
	mu    sync.Mutex
	fsl   *fslib.FsLib
	pclnt *procclnt.ProcClnt
	kclnt *kernelclnt.KernelClnt
	pdc   *protdevclnt.ProtDevClnt
}

func MakeUprocdMgr(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) *UprocdMgr {
	updm := &UprocdMgr{fsl: fsl, pclnt: pclnt}
	return updm
}

func (updm *UprocdMgr) StartUprocd(realm string) error {
	if updm.kclnt == nil {
		kclnt, err := kernelclnt.MakeKernelClnt(updm.fsl, sp.BOOT+"~local/")
		if err != nil {
			return err
		}
		updm.kclnt = kclnt
	}
	return updm.kclnt.Boot("uprocd", []string{"rootrealm"})
}

func (updm *UprocdMgr) lookupClnt(realm string) (*protdevclnt.ProtDevClnt, error) {
	updm.mu.Lock()
	defer updm.mu.Unlock()
	if updm.pdc == nil {
		if err := updm.StartUprocd(realm); err != nil {
			return nil, err
		}
		pn := path.Join(sp.PROCD, "~local", sp.UPROCDREL)
		pdc, err := protdevclnt.MkProtDevClnt(updm.fsl, pn)
		if err != nil {
			return nil, err
		}
		updm.pdc = pdc
	}
	return updm.pdc, nil
}

func (updm *UprocdMgr) MakeUProc(uproc *proc.Proc, realm string) error {
	pdc, err := updm.lookupClnt(realm)
	if err != nil {
		return err
	}
	req := &proto.RunRequest{
		ProcProto: uproc.GetProto(),
	}
	res := &proto.RunResult{}
	return pdc.RPC("UprocSrv.Run", req, res)
}
