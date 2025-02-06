package clnt

import (
	"sigmaos/apps/epcache"
	"sigmaos/apps/epcache/proto"
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

func (clnt *EndpointCacheClnt) RegisterEndpoint(name string, ep *sp.Tendpoint) error {
	var res proto.RegisterEndpointRep
	req := &proto.RegisterEndpointReq{
		ServiceName:   name,
		EndpointProto: ep.GetProto(),
	}
	err := clnt.rpcc.RPC("EPCacheSrv.RegisterEndpoint", req, &res)
	if err != nil {
		return err
	}
	return nil
}

func (clnt *EndpointCacheClnt) DeregisterEndpoint(name string, ep *sp.Tendpoint) error {
	var res proto.DeregisterEndpointRep
	req := &proto.DeregisterEndpointReq{
		ServiceName:   name,
		EndpointProto: ep.GetProto(),
	}
	err := clnt.rpcc.RPC("EPCacheSrv.DeregisterEndpoint", req, &res)
	if err != nil {
		return err
	}
	return nil
}

func (clnt *EndpointCacheClnt) GetEndpoints(name string, ep *sp.Tendpoint) error {
	var res proto.GetEndpointsRep
	req := &proto.GetEndpointsReq{
		ServiceName:   name,
		EndpointProto: ep.GetProto(),
	}
	err := clnt.rpcc.RPC("EPCacheSrv.GetEndpoints", req, &res)
	if err != nil {
		return err
	}
	return nil
}

func (clnt *EndpointCacheClnt) WaitForUpdates(name string, ep *sp.Tendpoint) error {
	var res proto.WaitForUpdatesRep
	req := &proto.WaitForUpdatesReq{
		ServiceName:   name,
		EndpointProto: ep.GetProto(),
	}
	err := clnt.rpcc.RPC("EPCacheSrv.WaitForUpdates", req, &res)
	if err != nil {
		return err
	}
	return nil
}
