package kernelclnt

import (
	"log"

	"sigmaos/fslib"
	"sigmaos/kernelsrv"
	"sigmaos/protdevclnt"
	//	sp "sigmaos/sigmap"
)

type KernelClnt struct {
	fsl *fslib.FsLib
	pdc *protdevclnt.ProtDevClnt
}

func (*kc KernelClnt) Boot(s string) error {
	var res BootResult
	req := kernel.BootRequest{Name: s}
	err := k.pdc.RPC("KernelSrv.Boot", req, &res)
	if err != nil {
		return err
	}
}

