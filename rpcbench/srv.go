package rpcbench

import (
	"time"

	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/rpcbench/proto"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

type Srv struct {
	pds    *sigmasrv.SigmaSrv
	tracer *tracing.Tracer
}

// Run starts the server
func RunRPCBenchSrv(path string, public bool) error {
	s := &Srv{}
	ssrv, err := sigmasrv.MakeSigmaSrvPublic(path, s, proc.GetProcEnv(), public)
	if err != nil {
		return err
	}

	s.tracer = tracing.Init("rpcsrv", proc.GetSigmaJaegerIP())

	defer s.tracer.Flush()

	return ssrv.RunServer()
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Srv) Sleep(ctx fs.CtxI, req proto.SleepRequest, res *proto.SleepResult) error {
	_, span := s.tracer.StartRPCSpan(&req, "Sleep")
	defer span.End()

	time.Sleep(10 * time.Millisecond)

	return nil
}
