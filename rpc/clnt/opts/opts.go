package opts

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/rpc/clnt/channel"
)

type RPCClntOptions struct {
	NewRPCChannel          channel.NewRPCChannelFn
	NewDelegatedRPCChannel channel.NewRPCChannelFn
}

func NewEmptyRPCClntOptions() *RPCClntOptions {
	return &RPCClntOptions{
		NewRPCChannel: func(string) (channel.RPCChannel, error) {
			db.DPrintf(db.ERROR, "RPC Channel constructor not set")
			return nil, fmt.Errorf("RPC Channel constructor not set")
		},
		NewDelegatedRPCChannel: func(string) (channel.RPCChannel, error) {
			db.DPrintf(db.RPCCHAN, "No delegated RPC channel supplied")
			return nil, nil
		},
	}
}

type RPCClntOption struct {
	Apply func(opts *RPCClntOptions)
}

func WithDelegatedRPCChannelConstructor(fn channel.NewRPCChannelFn) *RPCClntOption {
	return &RPCClntOption{
		Apply: func(opts *RPCClntOptions) {
			opts.NewDelegatedRPCChannel = fn
		},
	}
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

func WithDelegatedRPCChannel(ch channel.RPCChannel) *RPCClntOption {
	return WithDelegatedRPCChannelConstructor(func(pn string) (channel.RPCChannel, error) {
		return ch, nil
	})
}
