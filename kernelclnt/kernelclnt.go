package kernelclnt

import (
	"sigmaos/fslib"
	"sigmaos/kernelsrv/proto"
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

func (kc *KernelClnt) Boot(s string, args []string) error {
	var res proto.BootResult
	req := &proto.BootRequest{Name: s, Args: args}
	err := kc.pdc.RPC("KernelSrv.Boot", req, &res)
	if err != nil {
		return err
	}
	return nil
}

func (kc *KernelClnt) Kill(s string) error {
	var res proto.KillResult
	req := &proto.KillRequest{Name: s}
	err := kc.pdc.RPC("KernelSrv.Kill", req, &res)
	if err != nil {
		return err
	}
	return nil
}

func (kc *KernelClnt) Shutdown() error {
	var res proto.ShutdownResult
	req := &proto.ShutdownRequest{}
	err := kc.pdc.RPC("KernelSrv.Shutdown", req, &res)
	if err != nil {
		return err
	}
	return nil
}
