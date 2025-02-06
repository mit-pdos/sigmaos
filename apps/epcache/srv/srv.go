package srv

import (
	"sigmaos/api/fs"
	"sigmaos/apps/epcache"
	"sigmaos/apps/epcache/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
)

type EPCacheSrv struct {
}

func RunSrv() error {
	srv := &EPCacheSrv{}
	ssrv, err := sigmasrv.NewSigmaSrv(epcache.EPCACHE, srv, proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Err NewSigmaSrv: %v", err)
		return err
	}
	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.EPCACHE)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()

	return ssrv.RunServer()
}

func (srv *EPCacheSrv) RegisterEndpoint(ctx fs.CtxI, req proto.RegisterEndpointReq, rep *proto.RegisterEndpointRep) error {
	db.DPrintf(db.EPCACHE, "RegisterEndpoint %v", req)
	db.DFatalf("Unimplemented")
	return nil
}
