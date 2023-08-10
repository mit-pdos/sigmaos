package repl

import (
	proto "sigmaos/cache/replproto"
)

const (
	PLACEHOLDER_ADDR = "PLACEHOLDER_ADDR"
)

type Tapplyf func(*proto.ReplOpRequest, *proto.ReplOpReply) error

type Config interface {
	ReplAddr() string
	String() string
	MakeServer(Tapplyf) Server
}

type Server interface {
	Start()
	Process(*proto.ReplOpRequest, *proto.ReplOpReply) error
}
