package channel

import (
	"sigmaos/rpc"
	"sigmaos/sessp"
)

type NewRPCChannelFn func(pn string) (RPCChannel, error)

// RPC Channel interface. Encapsulates the abstraction of a blocking
// request-response and the transport layer below it.
type RPCChannel interface {
	SendReceive(sessp.IoVec, sessp.IoVec) error
	StatsSrv() (*rpc.RPCStatsSnapshot, error)
}
