package opts

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/rpc/clnt/channel"
)

type RPCClntOptions struct {
	NewRPCChannel channel.NewRPCChannelFn
}

func NewEmptyRPCClntOptions() *RPCClntOptions {
	return &RPCClntOptions{
		NewRPCChannel: func(string) (channel.RPCChannel, error) {
			db.DPrintf(db.ERROR, "RPC Channel constructor not set")
			return nil, fmt.Errorf("RPC Channel constructor not set")
		},
	}
}

type RPCClntOption struct {
	Apply func(opts *RPCClntOptions)
}

func WithRPCChannelConstructor(fn channel.NewRPCChannelFn) *RPCClntOption {
	return &RPCClntOption{
		Apply: func(opts *RPCClntOptions) {
			opts.NewRPCChannel = fn
		},
	}
}

func WithRPCChannel(ch channel.RPCChannel) *RPCClntOption {
	return WithRPCChannelConstructor(func(pn string) (channel.RPCChannel, error) {
		return ch, nil
	})
}
