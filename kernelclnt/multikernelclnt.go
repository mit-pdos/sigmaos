package kernelclnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpcdirclnt"
	sp "sigmaos/sigmap"
)

type MultiKernelClnt struct {
	*fslib.FsLib
	rpcdc *rpcdirclnt.RPCDirClnt
	done  int32
}

func NewMultiKernelClnt(fsl *fslib.FsLib) *MultiKernelClnt {
	return &MultiKernelClnt{
		FsLib: fsl,
		rpcdc: rpcdirclnt.NewRPCDirClnt(fsl, sp.BOOT, db.KERNELCLNT, db.KERNELCLNT_ERR),
	}
}

func (mkc *MultiKernelClnt) BootInRealm(kernelID string, realm sp.Trealm, s string, args []string) (sp.Tpid, error) {
	rpcc, err := mkc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return sp.NO_PID, err
	}
	return bootInRealm(rpcc, realm, s, args)
}

func (mkc *MultiKernelClnt) EvictKernelProc(kernelID string, pid sp.Tpid) error {
	rpcc, err := mkc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	return evictKernelProc(rpcc, pid)
}

func (mkc *MultiKernelClnt) GetKernelSrvs() ([]string, error) {
	return mkc.rpcdc.GetEntries()
}

func (mkc *MultiKernelClnt) StopWatching() {
	mkc.rpcdc.StopWatching()
}
