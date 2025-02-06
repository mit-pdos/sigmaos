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
