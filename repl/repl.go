package repl

import (
	"net"

	"ulambda/protsrv"
)

type Config interface {
	ReplAddr() string
	String() string
}

type Conn interface {
}

type Server interface {
	Init()
	MakeConn(protsrv.FsServer, net.Conn) Conn
}
