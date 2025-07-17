// Constructors for an RPC client operating over sigmap
package sigmap

import (
	spproxyclnt "sigmaos/proxy/sigmap/clnt"
	"sigmaos/rpc/clnt"
	"sigmaos/rpc/clnt/channel"
	"sigmaos/rpc/clnt/channel/spchannel"
	"sigmaos/rpc/clnt/opts"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	"sigmaos/sigmaclnt/fslib"

	db "sigmaos/debug"
)

func WithDelegatedSPProxyChannel(fsl *fslib.FsLib) *rpcclntopts.RPCClntOption {
	return &opts.RPCClntOption{
		Apply: func(opts *rpcclntopts.RPCClntOptions) {
			opts.NewDelegatedRPCChannel = func(pn string) (channel.RPCChannel, error) {
				db.DPrintf(db.RPCCHAN, "Use delegated chan? %v", fsl.ProcEnv().UseSPProxy)
				if fsl.ProcEnv().UseSPProxy {
					// Extract SPProxy channel & return it
					return fsl.FileAPI.(*spproxyclnt.SPProxyClnt).GetRPCChannel(), nil
				}
				return nil, nil
			}
		},
	}
}

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
