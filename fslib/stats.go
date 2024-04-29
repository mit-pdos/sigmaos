package fslib

import (
	gopath "path"

	"sigmaos/rpc"
	sp "sigmaos/sigmap"
	"sigmaos/stats"
)

func (fsl *FsLib) ReadStats(pn string) (*stats.StatsSnapshot, error) {
	st := &stats.StatsSnapshot{}
	err := fsl.GetFileJson(gopath.Join(pn, sp.STATSD), &st)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (fsl *FsLib) ReadRPCStats(pn string) (*rpc.RPCStatsSnapshot, error) {
	st := &rpc.RPCStatsSnapshot{}
	if err := fsl.GetFileJson(gopath.Join(pn, rpc.RPC, rpc.STATS), st); err != nil {
		return nil, err
	}
	return st, nil
}
