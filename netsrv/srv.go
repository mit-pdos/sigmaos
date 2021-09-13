package netsrv

import (
	"fmt"
	"log"
	"net"

	db "ulambda/debug"
	"ulambda/protsrv"
	"ulambda/repl"
)

type NetServer struct {
	addr       string
	fssrv      protsrv.FsServer
	wireCompat bool
	replicated bool
	replSrv    repl.Server
}

func MakeNetServer(address string, fssrv protsrv.FsServer) *NetServer {
	return MakeReplicatedNetServer(fssrv, address, false, false, nil)
}

func MakeNetServerWireCompatible(address string, fssrv protsrv.FsServer) *NetServer {
	return MakeReplicatedNetServer(fssrv, address, true, false, nil)
}

func MakeReplicatedNetServer(fs protsrv.FsServer, address string, wireCompat bool, replicated bool, config repl.Config) *NetServer {
	srv := &NetServer{"",
		fs,
		wireCompat, replicated,
		nil,
	}
	if replicated {
		db.DLPrintf("RSRV", "starting replicated server: %v\n", config)
		srv.replSrv = config.MakeServer()
		// Create and start the relay server listener
		db.DLPrintf("RSRV", "listen %v  myaddr %v\n", address, srv.addr)
		relayL, err := net.Listen("tcp", config.ReplAddr())
		if err != nil {
			log.Fatal("Replica server listen error:", err)
		}
		srv.replSrv.Init()
		// Start a server to listen for relay messages
		go srv.runsrv(relayL)
	}
	// Create and start the main server listener
	db.DLPrintf("9PCHAN", "listen %v  myaddr %v\n", address, srv.addr)
	var l net.Listener
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	srv.addr = l.Addr().String()
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
			log.Fatal("Accept error: ", err)
		}

		if !srv.replicated {
			MakeSrvConn(srv, conn)
		} else {
			db.DLPrintf("9PCHAN", "replsrv conn from %v -> %v\n", conn.RemoteAddr(), l.Addr())
			srv.replSrv.MakeConn(srv.fssrv, conn)
		}
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ addr: %v replicated: %v config: %v }", srv.addr, srv.replicated, srv.replSrv)
}
