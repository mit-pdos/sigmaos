package uprocclnt

import (
	"path"
	"sync"

	"sigmaos/fslib"
	"sigmaos/kernelclnt"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocdMgr struct {
	mu    sync.Mutex
	fsl   *fslib.FsLib
	kclnt *kernelclnt.KernelClnt
	pdcs  map[sp.Trealm]*protdevclnt.ProtDevClnt
}

func MakeUprocdMgr(fsl *fslib.FsLib) *UprocdMgr {
	updm := &UprocdMgr{fsl: fsl}
	updm.pdcs = make(map[sp.Trealm]*protdevclnt.ProtDevClnt)
	return updm
}

func (updm *UprocdMgr) startUprocd(realm sp.Trealm) error {
	if updm.kclnt == nil {
		kclnt, err := kernelclnt.MakeKernelClnt(updm.fsl, sp.BOOT+"~local/")
		if err != nil {
			return err
		}
		updm.kclnt = kclnt
	}
	return updm.kclnt.Boot("uprocd", []string{"rootrealm"})
}

func (updm *UprocdMgr) lookupClnt(realm sp.Trealm) (*protdevclnt.ProtDevClnt, error) {
	updm.mu.Lock()
	defer updm.mu.Unlock()
	pdc, ok := updm.pdcs[realm]
	if !ok {
		if err := updm.startUprocd(realm); err != nil {
			return nil, err
		}
		pn := path.Join(sp.PROCD, "~local", sp.UPROCDREL)
		c, err := protdevclnt.MkProtDevClnt(updm.fsl, pn)
		if err != nil {
			return nil, err
		}
		updm.pdcs[realm] = c
		pdc = c
	}
	return pdc, nil
}

func (updm *UprocdMgr) MakeUProc(uproc *proc.Proc, realm sp.Trealm) error {
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
