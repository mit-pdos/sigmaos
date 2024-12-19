package repl

import (
	proto "sigmaos/apps/kv/repl/proto"
)

type Tapplyf func(*proto.ReplOpReq, *proto.ReplOpRep) error

type Config interface {
	ReplAddr() string
	String() string
	NewServer(Tapplyf) Server
}

type Server interface {
	Start()
	Process(*proto.ReplOpReq, *proto.ReplOpRep) error
}
