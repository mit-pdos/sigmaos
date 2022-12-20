package repl

import (
	"sigmaos/sessp"
	"sigmaos/threadmgr"
)

const (
	PLACEHOLDER_ADDR = "PLACEHOLDER_ADDR"
)

type Config interface {
	ReplAddr() string
	String() string
	MakeServer(tm *threadmgr.ThreadMgr) Server
}

type Server interface {
	Start()
	Process(fc *sessp.FcallMsg)
}
