package clnt

import (
	"fmt"

	"sigmaos/apps/epcache"
	"sigmaos/apps/epcache/proto"
	db "sigmaos/debug"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type EndpointCacheClnt struct {
	fsl  *fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewEndpointCacheClnt(fsl *fslib.FsLib) (*EndpointCacheClnt, error) {
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, epcache.EPCACHE)
	if err != nil {
		return nil, err
	}
	return &EndpointCacheClnt{
		fsl:  fsl,
		rpcc: rpcc,
	}, nil
}

// Register a service's endpoint
func (clnt *EndpointCacheClnt) RegisterEndpoint(svcName string, ep *sp.Tendpoint) error {
	db.DPrintf(db.EPCACHECLNT, "RegisterEndpoint: %v -> %v", svcName, ep)
	var res proto.RegisterEndpointRep
	req := &proto.RegisterEndpointReq{
		ServiceName:   svcName,
		EndpointProto: ep.GetProto(),
	}
	err := clnt.rpcc.RPC("EPCacheSrv.RegisterEndpoint", req, &res)
	if err != nil {
		db.DPrintf(db.EPCACHECLNT_ERR, "Err Register: %v", err)
		return err
	}
	if !res.OK {
		return fmt.Errorf("Register failed")
	}
	db.DPrintf(db.EPCACHECLNT, "RegisterEndpoint ok: %v -> %v", svcName, ep)
	return nil
}

// Deregister a service endpoint
func (clnt *EndpointCacheClnt) DeregisterEndpoint(svcName string, ep *sp.Tendpoint) error {
	db.DPrintf(db.EPCACHECLNT, "DeregisterEndpoint done: %v -> %v", svcName, ep)
	var res proto.DeregisterEndpointRep
	req := &proto.DeregisterEndpointReq{
		ServiceName:   svcName,
		EndpointProto: ep.GetProto(),
	}
	err := clnt.rpcc.RPC("EPCacheSrv.DeregisterEndpoint", req, &res)
	if err != nil {
		db.DPrintf(db.EPCACHECLNT_ERR, "Err Deregister: %v", err)
		return err
	}
	if !res.OK {
		return fmt.Errorf("Deregister failed")
	}
	db.DPrintf(db.EPCACHECLNT, "DeregisterEndpoint ok: %v -> %v", svcName, ep)
	return nil
}

// Get set of endpoints which back a service. If v == NO_VERSION, return the
// current set of endpoints immediately. Otherwise, block until the version of
// the service's set of endpoints is >v, and then return those endpoints.
func (clnt *EndpointCacheClnt) GetEndpoints(svcName string, v1 epcache.Tversion) ([]*sp.Tendpoint, epcache.Tversion, error) {
	db.DPrintf(db.EPCACHECLNT, "GetEndpoints: %v %v", svcName, v1)
	var res proto.GetEndpointsRep
	req := &proto.GetEndpointsReq{
		ServiceName: svcName,
		Version:     uint64(v1),
	}
	err := clnt.rpcc.RPC("EPCacheSrv.GetEndpoints", req, &res)
	if err != nil {
		db.DPrintf(db.EPCACHECLNT_ERR, "Err GetEndpoint: %v", err)
		return nil, epcache.NO_VERSION, err
	}
	v2 := epcache.Tversion(res.Version)
	eps := make([]*sp.Tendpoint, len(res.EndpointProtos))
	for i := 0; i < len(eps); i++ {
		eps[i] = sp.NewEndpointFromProto(res.EndpointProtos[i])
	}
	db.DPrintf(db.EPCACHECLNT, "GetEndpoints ok: %v %v -> %v %v", svcName, v1, v2, eps)
	return eps, v2, nil
}
