package kernelclnt

import (
	"sigmaos/fslib"
	"sigmaos/kernelsrv"
	"sigmaos/protdevclnt"
	//	sp "sigmaos/sigmap"
)

type KernelClnt struct {
	fsl *fslib.FsLib
	pdc *protdevclnt.ProtDevClnt
}

func MakeKernelClnt(fsl *fslib.FsLib, pn string) (*KernelClnt, error) {
	pdc, err := protdevclnt.MkProtDevClnt(fsl, pn)
	if err != nil {
		return nil, err
	}
	return &KernelClnt{fsl, pdc}, nil
}

func (kc *KernelClnt) Boot(s string) error {
	var res kernelsrv.BootResult
	req := &kernelsrv.BootRequest{Name: s}
	err := kc.pdc.RPC("KernelSrv.Boot", req, &res)
	if err != nil {
		return err
	}
	return nil
}

func (kc *KernelClnt) Kill(s string) error {
	var res kernelsrv.KillResult
	req := &kernelsrv.KillRequest{Name: s}
	err := kc.pdc.RPC("KernelSrv.Kill", req, &res)
	if err != nil {
		return err
	}
	return nil
}
