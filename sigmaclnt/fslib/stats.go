package fslib

import (
	"path/filepath"

	"sigmaos/rpc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/stats"
	"sigmaos/util/spstats"
)

func (fsl *FsLib) ReadSrvStats(pn sp.Tsigmapath) (*stats.SrvStatsSnapshot, error) {
	st := &stats.SrvStatsSnapshot{}
	err := fsl.GetFileJson(filepath.Join(pn, sp.STATSD), &st)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (fsl *FsLib) ReadRPCStats(pn sp.Tsigmapath) (*rpc.RPCStatsSnapshot, error) {
	st := &rpc.RPCStatsSnapshot{}
	if err := fsl.GetFileJson(filepath.Join(pn, rpc.RPC, rpc.STATS), st); err != nil {
		return nil, err
	}
	return st, nil
}

// Read named's per-path stats. The snapshot type is
// spstats.TcounterSnapshot, which fsetcd aliases as PstatsSnapshot: naming
// fsetcd's type here would make every proc that links fslib link the etcd
// client too.
func (fsl *FsLib) ReadPstats() (*spstats.TcounterSnapshot, error) {
	st := spstats.NewTcounterSnapshot()
	err := fsl.GetFileJson(filepath.Join(sp.NAMED, sp.PSTATSD), &st)
	if err != nil {
		return nil, err
	}
	return st, nil
}
