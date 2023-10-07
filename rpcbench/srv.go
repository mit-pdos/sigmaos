package rpcbench

import (
	"time"

	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/rpcbench/proto"
	"sigmaos/sigmasrv"
)

type Srv struct {
	pds *sigmasrv.SigmaSrv
}

// Run starts the server
func RunRPCBenchSrv(path string, public bool) error {
	s := &Srv{}
	ssrv, err := sigmasrv.NewSigmaSrvPublic(path, s, proc.GetProcEnv(), public)
	if err != nil {
		return err
	}

	return ssrv.RunServer()
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Srv) Sleep(ctx fs.CtxI, req proto.SleepRequest, res *proto.SleepResult) error {

	time.Sleep(10 * time.Millisecond)

	return nil
}

func (s *Srv) NoOp(ctx fs.CtxI, req proto.NoOpRequest, res *proto.NoOpResult) error {
	return nil
}
