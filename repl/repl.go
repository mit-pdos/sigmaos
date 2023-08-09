package repl

import (
	proto "sigmaos/repl/proto"
)

const (
	PLACEHOLDER_ADDR = "PLACEHOLDER_ADDR"
)

type Config interface {
	ReplAddr() string
	String() string
	MakeServer(chan *proto.ReplRequest) Server
}

type Server interface {
	Start()
	Process(*proto.ReplRequest)
}
