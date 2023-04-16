package rpcbench

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	"sigmaos/rpcbench/proto"
	"sigmaos/tracing"
)

type Srv struct {
	pds    *protdevsrv.ProtDevSrv
	tracer *tracing.Tracer
}

// Run starts the server
func RunRPCBenchSrv(path string, public bool) error {
	s := &Srv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(path, s, public)
	if err != nil {
		return err
	}

	p, err := perf.MakePerf(perf.RPC_BENCH_SRV)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	s.tracer = tracing.Init("rpcsrv", proc.GetSigmaJaegerIP())

	defer s.tracer.Flush()

	return pds.RunServer()
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Srv) Sleep(ctx fs.CtxI, req proto.SleepRequest, res *proto.SleepResult) error {
	_, span := s.tracer.StartRPCSpan(&req, "Sleep")
	defer span.End()

	time.Sleep(10 * time.Millisecond)

	return nil
}
