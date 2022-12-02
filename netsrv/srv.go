package netsrv

import (
	"bufio"
	"fmt"
	"net"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type MarshalF func(fcall.Fcall, *bufio.Writer) *fcall.Err
type UnmarshalF func([]byte) (fcall.Fcall, *fcall.Err)

type NetServer struct {
	addr      string
	sesssrv   sp.SessServer
	marshal   MarshalF
	unmarshal UnmarshalF
}

func MakeNetServer(ss sp.SessServer, address string, m MarshalF, u UnmarshalF) *NetServer {
	srv := &NetServer{"",
		ss,
		m,
		u,
	}
	// Create and start the main server listener
	var l net.Listener
	l, err := net.Listen("tcp", address)
	if err != nil {
		db.DFatalf("Listen error: %v", err)
	}
	srv.addr = l.Addr().String()
	db.DPrintf("NETSRV", "listen %v myaddr %v\n", address, srv.addr)
	go srv.runsrv(l)
	return srv
}

func (srv *NetServer) MyAddr() string {
	return srv.addr
}

func (srv *NetServer) runsrv(l net.Listener) {
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DFatalf("%v: Accept error: %v", proc.GetName(), err)
		}

		MakeSrvConn(srv, conn)
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ addr: %v }", srv.addr)
}
