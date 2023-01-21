package sigmaclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

type SigmaClnt struct {
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MkSigmaClntProc(name string, ip string, namedAddr []string) (*SigmaClnt, error) {
	fsl, err := fslib.MakeFsLibAddr(name, ip, namedAddr)
	if err != nil {
		return nil, err
	}
	pclnt := procclnt.MakeProcClntInit(proc.GenPid(), fsl, name, namedAddr)
	return &SigmaClnt{fsl, pclnt}, nil
}

func MkSigmaClnt(name string) (*SigmaClnt, error) {
	fsl, err := fslib.MakeFsLib(name)
	if err != nil {
		db.DFatalf("MkSigmaClnt: %v", err)
	}
	pclnt := procclnt.MakeProcClnt(fsl)
	return &SigmaClnt{fsl, pclnt}, nil
}
