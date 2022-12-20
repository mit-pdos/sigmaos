package repl

import (
	"sigmaos/fcall"
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
	Process(fc *fcall.FcallMsg)
}
