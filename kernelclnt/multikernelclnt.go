package kernelclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/unionrpcclnt"
)

type MultiKernelClnt struct {
	*fslib.FsLib
	urpcc *unionrpcclnt.UnionRPCClnt
	done  int32
}

func NewMultiKernelClnt(fsl *fslib.FsLib) *MultiKernelClnt {
	return &MultiKernelClnt{
		FsLib: fsl,
		urpcc: unionrpcclnt.NewUnionRPCClnt(fsl, sp.BOOT, db.KERNELCLNT, db.KERNELCLNT_ERR),
	}
}

func (mkc *MultiKernelClnt) BootInRealm(kernelID string, realm sp.Trealm, s string, args []string) (sp.Tpid, error) {
	rpcc, err := mkc.urpcc.GetClnt(kernelID)
	if err != nil {
		return sp.NO_PID, err
	}
	return bootInRealm(rpcc, realm, s, args)
}

func (mkc *MultiKernelClnt) EvictKernelProc(kernelID string, pid sp.Tpid) error {
	rpcc, err := mkc.urpcc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	return evictKernelProc(rpcc, pid)
}

func (mkc *MultiKernelClnt) GetKernelSrvs() ([]string, error) {
	return mkc.urpcc.GetSrvs()
}

func (mkc *MultiKernelClnt) StopWatching() {
	mkc.urpcc.StopWatching()
}
