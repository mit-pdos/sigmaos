package netsrv

import (
	"fmt"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
)

type NetServer struct {
	addr       string
	sesssrv    np.SessServer
	wireCompat bool
}

func MakeNetServer(sesssrv np.SessServer, address string) *NetServer {
	return makeNetServer(sesssrv, address, false)
}

func MakeNetServerWireCompatible(address string, sesssrv np.SessServer) *NetServer {
	return makeNetServer(sesssrv, address, true)
}

func makeNetServer(ss np.SessServer, address string, wireCompat bool) *NetServer {
	srv := &NetServer{"",
		ss,
		wireCompat,
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
