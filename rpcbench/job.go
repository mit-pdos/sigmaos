package rpcbench

import (
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/sigmaclnt"
)

const (
	SRVNAME = "rpcbenchsrv"
)

type RPCBenchJob struct {
	*sigmaclnt.SigmaClnt
	pids []proc.Tpid
}

func MakeRPCBenchJob(sc *sigmaclnt.SigmaClnt, jobpath string, mcpu proc.Tmcpu, public bool) (*RPCBenchJob, error) {
	var err error

	pids := make([]proc.Tpid, 0, 1)

	p := proc.MakeProc(SRVNAME, []string{jobpath, strconv.FormatBool(public)})
	p.SetMcpu(mcpu)
	if _, errs := sc.SpawnBurst([]*proc.Proc{p}, 2); len(errs) > 0 {
		db.DFatalf("Error burst-spawnn proc %v: %v", p, errs)
		return nil, err
	}
	if err = sc.WaitStart(p.GetPid()); err != nil {
		db.DFatalf("Error spawn proc %v: %v", p, err)
		return nil, err
	}
	pids = append(pids, p.GetPid())

	return &RPCBenchJob{sc, pids}, nil
}

func (rj *RPCBenchJob) Stop() error {
	for _, pid := range rj.pids {
		if err := rj.Evict(pid); err != nil {
			return err
		}
		if _, err := rj.WaitExit(pid); err != nil {
			return err
		}
	}
	return nil
}

func (rj *RPCBenchJob) StatsSrv() ([]*rpc.SigmaRPCStats, error) {
	return nil, nil
}
