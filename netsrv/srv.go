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
	fssrv      np.FsServer
	wireCompat bool
}

func MakeNetServer(fssrv np.FsServer, address string) *NetServer {
	return makeNetServer(fssrv, address, false)
}

func MakeNetServerWireCompatible(address string, fssrv np.FsServer) *NetServer {
	return makeNetServer(fssrv, address, true)
}

func makeNetServer(fs np.FsServer, address string, wireCompat bool) *NetServer {
	srv := &NetServer{"",
		fs,
		wireCompat,
	}
	// Create and start the main server listener
	var l net.Listener
	l, err := net.Listen("tcp", address)
	if err != nil {
		db.DFatalf("Listen error:", err)
	}
	srv.addr = l.Addr().String()
	db.DPrintf("9PCHAN", "listen %v myaddr %v\n", address, srv.addr)
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
