package netsrv

import (
	"log"
	"net"

	db "ulambda/debug"
	"ulambda/protsrv"
	"ulambda/repl"
)

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
