package kernelclnt

import (
	"strings"

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

func NewMultiKernelClnt(fsl *fslib.FsLib, lsel, esel db.Tselector) *MultiKernelClnt {
	return &MultiKernelClnt{
		FsLib: fsl,
		rpcdc: rpcdirclnt.NewRPCDirClntFilter(fsl, sp.BOOT, lsel, esel, sp.SIGMACLNTDKERNEL),
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

func (mkc *MultiKernelClnt) GetGeneralKernels() ([]string, error) {
	es, err := mkc.rpcdc.GetEntries()
	if err != nil {
		return nil, err
	}
	kids := make([]string, 0, len(es))
	for _, e := range es {
		if !strings.HasPrefix(e, sp.SIGMACLNTDKERNEL) {
			kids = append(kids, e)
		}
	}
	return kids, nil
}

func (mkc *MultiKernelClnt) StopWatching() {
	mkc.rpcdc.StopWatching()
}
