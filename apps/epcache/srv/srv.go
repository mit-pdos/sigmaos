package srv

import (
	"sync"

	"sigmaos/api/fs"
	"sigmaos/apps/epcache"
	"sigmaos/apps/epcache/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
)

type EPCacheSrv struct {
	mu   sync.Mutex
	svcs map[string]*svc
}

type svc struct {
	cond *sync.Cond
	name string
	v    epcache.Tversion
	eps  map[string]*sp.Tendpoint
}

func newSvc(mu sync.Locker, name string) *svc {
	return &svc{
		cond: sync.NewCond(mu),
		name: name,
		v:    epcache.Tversion(1),
		eps:  make(map[string]*sp.Tendpoint),
	}
}

func (svc *svc) add(ep *sp.Tendpoint) {
	// Store new EP
	svc.eps[ep.String()] = ep
	// Bump version
	svc.v++
	db.DPrintf(db.EPCACHE, "Add svc %v %v ep %v result:", svc.name, svc.v, ep, svc.eps)
	// Wake up waiters
	svc.cond.Broadcast()
}

func (svc *svc) del(ep *sp.Tendpoint) bool {
	key := ep.String()
	_, ok := svc.eps[key]
	// Delete EP
	if !ok {
		db.DPrintf(db.EPCACHE_ERR, "Try del unknown EP svc %v ep %v", svc.name, ep)
		return false
	}
	delete(svc.eps, key)
	// Bump version
	svc.v++
	db.DPrintf(db.EPCACHE, "Del svc %v %v ep %v result: %v", svc.name, svc.v, ep, svc.eps)
	// Wake up waiters
	svc.cond.Broadcast()
	return true
}

func RunSrv() error {
	srv := &EPCacheSrv{
		svcs: make(map[string]*svc),
	}
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

// Caller holds lock
func (srv *EPCacheSrv) addEP(svcName string, ep *sp.Tendpoint) {
	var svc *svc
	var ok bool
	svc, ok = srv.svcs[svcName]
	if !ok {
		svc = newSvc(&srv.mu, svcName)
		srv.svcs[svcName] = svc
	}
	key := ep.String()
	svc.eps[key] = ep
}

// Caller holds lock
func (srv *EPCacheSrv) delEP(svcName string, ep *sp.Tendpoint) bool {
	svc, ok := srv.svcs[svcName]
	if !ok {
		db.DPrintf(db.EPCACHE_ERR, "Try del unknown SVC svc %v ep %v", svc.name, ep)
		return false
	}
	return svc.del(ep)
}

func (srv *EPCacheSrv) RegisterEndpoint(ctx fs.CtxI, req proto.RegisterEndpointReq, rep *proto.RegisterEndpointRep) error {
	db.DPrintf(db.EPCACHE, "RegisterEndpoint %v", req)

	srv.mu.Lock()
	defer srv.mu.Unlock()

	srv.addEP(req.ServiceName, sp.NewEndpointFromProto(req.EndpointProto))
	rep.OK = true
	db.DPrintf(db.EPCACHE, "RegisterEndpoint done req:%v rep:%v", req, rep)
	return nil
}

func (srv *EPCacheSrv) DeregisterEndpoint(ctx fs.CtxI, req proto.DeregisterEndpointReq, rep *proto.DeregisterEndpointRep) error {
	db.DPrintf(db.EPCACHE, "DeregisterEndpoint %v", req)

	srv.mu.Lock()
	defer srv.mu.Unlock()

	rep.OK = srv.delEP(req.ServiceName, sp.NewEndpointFromProto(req.EndpointProto))
	db.DPrintf(db.EPCACHE, "DeregisterEndpoint done req:%v rep:%v", req, rep)
	return nil
}

func (srv *EPCacheSrv) GetEndpoints(ctx fs.CtxI, req proto.GetEndpointsReq, rep *proto.GetEndpointsRep) error {
	db.DPrintf(db.EPCACHE, "GetEndpoints %v", req)

	srv.mu.Lock()
	defer srv.mu.Unlock()

	db.DFatalf("Unimplemented")
	db.DPrintf(db.EPCACHE, "GetEndpoints done req:%v rep:%v", req, rep)
	return nil
}
