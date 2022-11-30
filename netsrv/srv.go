package netsrv

import (
	"fmt"
	"net"

	db "sigmaos/debug"
	np "sigmaos/sigmap"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type NetServer struct {
	addr      string
	sesssrv   sp.SessServer
	sesssrv9p np.SessServer
}

func MakeNetServer(sesssrv sp.SessServer, address string) *NetServer {
	return makeNetServer(sesssrv, address, nil)
}

func MakeNetServer9p(address string, sesssrv np.SessServer) *NetServer {
	return makeNetServer(nil, address, sesssrv)
}

func makeNetServer(ss sp.SessServer, address string, npss np.SessServer) *NetServer {
	srv := &NetServer{"",
		ss,
		npss,
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
