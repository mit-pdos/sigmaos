package srv

import (
	"fmt"
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
	svc, ok := srv.svcs[svcName]
	if !ok {
		svc = newSvc(&srv.mu, svcName)
		srv.svcs[svcName] = svc
	}
	svc.add(ep)
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

// Caller holds lock
func (srv *EPCacheSrv) getEPs(svcName string, v1 epcache.Tversion) (epcache.Tversion, []*sp.TendpointProto, error) {
	svc, ok := srv.svcs[svcName]
	if !ok {
		db.DPrintf(db.EPCACHE_ERR, "Try get unknown SVC svc %v %v", svcName, v1)
		return epcache.NO_VERSION, nil, fmt.Errorf("Unkown svc %v", svcName)
	}
	if v1 != epcache.NO_VERSION {
		// If this is a versioned get, wait until v > v1
		for svc.v < v1 {
			svc.cond.Wait()
		}
	}
	// Either this was an unversioned get, or the wait has terminated. It is now
	// safe to return
	eps := make([]*sp.TendpointProto, 0, len(svc.eps))
	for _, ep := range svc.eps {
		eps = append(eps, ep.GetProto())
	}
	return svc.v, eps, nil
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

	v, eps, err := srv.getEPs(req.ServiceName, epcache.Tversion(req.Version))
	if err != nil {
		return err
	}
	rep.Version = uint64(v)
	rep.EndpointProtos = eps
	db.DPrintf(db.EPCACHE, "GetEndpoints done req:%v rep:%v", req, rep)
	return nil
}
