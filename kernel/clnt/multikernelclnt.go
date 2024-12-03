package clnt

import (
	db "sigmaos/debug"
	"sigmaos/fslib"
	shardedsvcrpcclnt "sigmaos/rpc/shardedsvc/clnt"
	sp "sigmaos/sigmap"
)

type MultiKernelClnt struct {
	*fslib.FsLib
	rpcdc *shardedsvcrpcclnt.ShardedSvcRPCClnt
	done  int32
}

func NewMultiKernelClnt(fsl *fslib.FsLib, lsel, esel db.Tselector) *MultiKernelClnt {
	return &MultiKernelClnt{
		FsLib: fsl,
		rpcdc: shardedsvcrpcclnt.NewShardedSvcRPCClntFilter(fsl, sp.BOOT, lsel, esel, []string{sp.SPPROXYDKERNEL, sp.BESCHEDKERNEL}),
	}
}

func (mkc *MultiKernelClnt) BootInRealm(kernelID string, realm sp.Trealm, s string, args []string) (sp.Tpid, error) {
	rpcc, err := mkc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return sp.NO_PID, err
	}
	return bootInRealm(rpcc, realm, s, args, []string{})
}

func (mkc *MultiKernelClnt) EvictKernelProc(kernelID string, pid sp.Tpid) error {
	rpcc, err := mkc.rpcdc.GetClnt(kernelID)
	if err != nil {
		return err
	}
	return evictKernelProc(rpcc, pid)
}

func (mkc *MultiKernelClnt) GetGeneralKernels() ([]string, error) {
	return mkc.rpcdc.WaitTimedGetEntriesN(1)
}

func (mkc *MultiKernelClnt) StopWatching() {
	mkc.rpcdc.StopWatching()
}
