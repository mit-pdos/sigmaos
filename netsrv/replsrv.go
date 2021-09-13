package netsrv

import (
	"log"
	"net"

	db "ulambda/debug"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/replchain"
)

func MakeReplicatedNetServer(fs protsrv.FsServer, address string, wireCompat bool, replicated bool, config repl.Config) *NetServer {
	var replState *replchain.ReplState
	var cfg *replchain.NetServerReplConfig
	if replicated {
		cfg = config.(*replchain.NetServerReplConfig)
		db.DLPrintf("RSRV", "starting replicated server: %v\n", cfg)
		replState = replchain.MakeReplState(cfg)
	}
	srv := &NetServer{"",
		fs,
		wireCompat, replicated,
		replState,
	}
	if replicated {
		// Create and start the relay server listener
		db.DLPrintf("RSRV", "listen %v  myaddr %v\n", address, srv.addr)
		relayL, err := net.Listen("tcp", cfg.RelayAddr)
		if err != nil {
			log.Fatal("Relay listen error:", err)
		}
		// Start a server to listen for relay messages
		go srv.runsrv(relayL)
		srv.replState.Init()
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
