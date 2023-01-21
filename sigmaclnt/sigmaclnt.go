package sigmaclnt

import (
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

type SigmaClnt struct {
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MkSigmaClnt(name string, ip string, namedAddr []string) (*SigmaClnt, error) {
	fsl, err := fslib.MakeFsLibAddr(name, ip, namedAddr)
	if err != nil {
		return nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, name, namedAddr)
	return &SigmaClnt{fsl, pclnt}, nil
}
