package srv

import (
	"fmt"
	"sync"

	"sigmaos/api/fs"
	"sigmaos/apps/epcache"
	"sigmaos/apps/epcache/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
)

type EPCacheSrv struct {
	mu   sync.Mutex
	svcs map[string]*svc
}

type svc struct {
	cond      *sync.Cond
	name      string
	v         epcache.Tversion
	instances map[string]*proto.Instance
}

func newSvc(mu sync.Locker, name string) *svc {
	return &svc{
		cond:      sync.NewCond(mu),
		name:      name,
		v:         epcache.Tversion(1),
		instances: make(map[string]*proto.Instance),
	}
}

func (svc *svc) add(i *proto.Instance) {
	// Store new EP
	svc.instances[i.GetID()] = i
	// Bump version
	svc.v++
	db.DPrintf(db.EPCACHE, "Add svc %v %v i %v result:", svc.name, svc.v, i, svc.instances)
	// Wake up waiters
	svc.cond.Broadcast()
}

func (svc *svc) del(instanceID string) bool {
	_, ok := svc.instances[instanceID]
	// Delete EP
	if !ok {
		db.DPrintf(db.EPCACHE_ERR, "Try del unknown EP svc %v id %v", svc.name, instanceID)
		return false
	}
	delete(svc.instances, instanceID)
	// Bump version
	svc.v++
	db.DPrintf(db.EPCACHE, "Del svc %v %v instanceID %v result: %v", svc.name, svc.v, instanceID, svc.instances)
	// Wake up waiters
	svc.cond.Broadcast()
	return true
}

func (svc *svc) get(v1 epcache.Tversion) []*proto.Instance {
	db.DPrintf(db.EPCACHE, "Get svc %v current:%v requested:%v", svc.name, svc.v, v1)
	if v1 != epcache.NO_VERSION {
		// If this is a versioned get, wait until v > v1
		for svc.v <= v1 {
			svc.cond.Wait()
		}
	}
	db.DPrintf(db.EPCACHE, "Get svc %v %v > %v result: %v", svc.name, svc.v, v1, svc.instances)
	is := make([]*proto.Instance, 0, len(svc.instances))
	for _, i := range svc.instances {
		is = append(is, i)
	}
	return is
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
func (srv *EPCacheSrv) addInstance(svcName string, i *proto.Instance) {
	svc, ok := srv.svcs[svcName]
	if !ok {
		svc = newSvc(&srv.mu, svcName)
		srv.svcs[svcName] = svc
	}
	svc.add(i)
}

// Caller holds lock
func (srv *EPCacheSrv) delInstance(svcName string, instanceID string) bool {
	svc, ok := srv.svcs[svcName]
	if !ok {
		db.DPrintf(db.EPCACHE_ERR, "Try del unknown SVC svc %v instanceID %v", svc.name, instanceID)
		return false
	}
	return svc.del(instanceID)
}

// Caller holds lock
func (srv *EPCacheSrv) getInstances(svcName string, v1 epcache.Tversion) (epcache.Tversion, []*proto.Instance, error) {
	svc, ok := srv.svcs[svcName]
	if !ok {
		db.DPrintf(db.EPCACHE_ERR, "Try get unknown SVC svc %v %v", svcName, v1)
		return epcache.NO_VERSION, nil, fmt.Errorf("Unkown svc %v", svcName)
	}
	instances := svc.get(v1)
	// Either this was an unversioned get, or the wait has terminated. It is now
	// safe to return
	return svc.v, instances, nil
}

func (srv *EPCacheSrv) RegisterEndpoint(ctx fs.CtxI, req proto.RegisterEndpointReq, rep *proto.RegisterEndpointRep) error {
	db.DPrintf(db.EPCACHE, "RegisterEndpoint %v", req)

	srv.mu.Lock()
	defer srv.mu.Unlock()

	srv.addInstance(req.ServiceName, req.Instance)
	rep.OK = true
	db.DPrintf(db.EPCACHE, "RegisterEndpoint done req:%v rep:%v", req, rep)
	return nil
}

func (srv *EPCacheSrv) DeregisterEndpoint(ctx fs.CtxI, req proto.DeregisterEndpointReq, rep *proto.DeregisterEndpointRep) error {
	db.DPrintf(db.EPCACHE, "DeregisterEndpoint %v", req)

	srv.mu.Lock()
	defer srv.mu.Unlock()

	rep.OK = srv.delInstance(req.ServiceName, req.InstanceID)
	db.DPrintf(db.EPCACHE, "DeregisterEndpoint done req:%v rep:%v", req, rep)
	return nil
}

func (srv *EPCacheSrv) GetEndpoints(ctx fs.CtxI, req proto.GetEndpointsReq, rep *proto.GetEndpointsRep) error {
	db.DPrintf(db.EPCACHE, "GetEndpoints %v", req)

	srv.mu.Lock()
	defer srv.mu.Unlock()

	v, instances, err := srv.getInstances(req.ServiceName, epcache.Tversion(req.Version))
	if err != nil {
		return err
	}
	rep.Version = uint64(v)
	rep.Instances = instances
	db.DPrintf(db.EPCACHE, "GetEndpoints done req:%v rep:%v", req, rep)
	return nil
}
