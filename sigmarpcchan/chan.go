package sigmarpcchan

import (
	"path/filepath"
	"sync/atomic"

	"sigmaos/fslib"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	"sigmaos/sessdevclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type SigmaRPCCh struct {
	fsls []*fslib.FsLib
	fds  []int
	pn   string
	idx  atomic.Int32
}

func SigmaRPCChanFactory(fsls []*fslib.FsLib) rpcclnt.NewRPCChFn {
	return func(pn string) (rpcclnt.RPCCh, error) {
		return NewSigmaRPCCh(fsls, pn)
	}
}

func NewSigmaRPCChEndpoint(fsls []*fslib.FsLib, pn string, mnt *sp.Tendpoint) (rpcclnt.RPCCh, error) {
	for _, fsl := range fsls {
		err := fsl.MountTree(mnt, rpc.RPC, filepath.Join(pn, rpc.RPC))
		if err != nil {
			return nil, err
		}
	}
	return NewSigmaRPCCh(fsls, pn)
}

func NewSigmaRPCCh(fsls []*fslib.FsLib, pn string) (rpcclnt.RPCCh, error) {
	rpcch := &SigmaRPCCh{
		fsls: make([]*fslib.FsLib, 0, len(fsls)),
		fds:  make([]int, 0, len(fsls)),
		pn:   pn,
	}
	sdc, err := sessdevclnt.NewSessDevClnt(fsls[0], filepath.Join(pn, rpc.RPC))
	if err != nil {
		return nil, err
	}
	for _, fsl := range fsls {
		rpcch.fsls = append(rpcch.fsls, fsl)
		fd, err := fsl.Open(sdc.DataPn(), sp.ORDWR)
		if err != nil {
			return nil, err
		}
		rpcch.fds = append(rpcch.fds, fd)
	}
	return rpcch, nil
}

func (ch *SigmaRPCCh) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
	idx := int(ch.idx.Add(1))
	err := ch.fsls[idx%len(ch.fsls)].WriteRead(ch.fds[idx%len(ch.fds)], iniov, outiov)
	if err != nil {
		return err
	}
	return nil
}

func (ch *SigmaRPCCh) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	stats, err := ch.fsls[0].ReadRPCStats(ch.pn)
	if err != nil {
		return nil, err
	}
	return stats, nil
}
