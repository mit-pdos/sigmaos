package netconn

import (
	"sigmaos/rpc/clnt"
	"sigmaos/rpc/clnt/channel/rpcchannel"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	sp "sigmaos/sigmap"
)

func NewTCPRPCClnt(name string, ep *sp.Tendpoint) (*clnt.RPCClnt, error) {
	ch, err := rpcchannel.NewTCPConnChannel(ep)
	if err != nil {
		return nil, err
	}
	return clnt.NewRPCClnt(name, rpcclntopts.WithRPCChannel(ch))
}

func NewUnixRPCClnt(name, pn string) (*clnt.RPCClnt, error) {
	ch, err := rpcchannel.NewUnixConnChannel(pn)
	if err != nil {
		return nil, err
	}
	return clnt.NewRPCClnt(name, rpcclntopts.WithRPCChannel(ch))
}
