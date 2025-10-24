package metrics

import (
	rpcclnt "sigmaos/rpc/clnt"
	rpcproto "sigmaos/rpc/proto"
)

type RPCMetricsClnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewRPCMetricsClnt(rpcc *rpcclnt.RPCClnt) *RPCMetricsClnt {
	return &RPCMetricsClnt{
		rpcc: rpcc,
	}
}

func (rmc *RPCMetricsClnt) GetMetrics() (*rpcproto.MetricsRep, error) {
	req := &rpcproto.MetricsReq{}
	rep := &rpcproto.MetricsRep{}
	if err := rmc.rpcc.RPC("RPCSrv.GetMetrics", req, rep); err != nil {
		return nil, err
	}
	return rep, nil
}
