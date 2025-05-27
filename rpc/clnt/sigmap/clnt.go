// Constructors for an RPC client operating over sigmap
package sigmap

import (
	"sigmaos/rpc/clnt"
	"sigmaos/rpc/clnt/channel"
	"sigmaos/rpc/clnt/channel/spchannel"
	"sigmaos/rpc/clnt/opts"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	"sigmaos/sigmaclnt/fslib"
)

func WithSPChannel(fsl *fslib.FsLib) *rpcclntopts.RPCClntOption {
	return &opts.RPCClntOption{
		Apply: func(opts *rpcclntopts.RPCClntOptions) {
			opts.NewRPCChannel = func(pn string) (channel.RPCChannel, error) {
				return spchannel.NewSPChannel(fsl, pn)
			}
		},
	}
}

func NewRPCClnt(fsl *fslib.FsLib, pn string) (*clnt.RPCClnt, error) {
	return clnt.NewRPCClnt(pn, WithSPChannel(fsl))
}
