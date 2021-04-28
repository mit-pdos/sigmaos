package npsrv

import (
	"fmt"
	"log"
	"net"

	db "ulambda/debug"
	"ulambda/npclnt"
)

type NpConn interface {
	Connect(net.Conn) NpAPI
}

type NpServer struct {
	npc        NpConn
	addr       string
	replicated bool
	replConfig *NpServerReplConfig
}

type NpServerReplConfig struct {
	HeadAddr string
	TailAddr string
	PrevAddr string
	NextAddr string
	HeadChan *npclnt.NpChan
	TailChan *npclnt.NpChan
	PrevChan *npclnt.NpChan
	NextChan *npclnt.NpChan
	*npclnt.NpClnt
}

func MakeNpServer(npc NpConn, address string) *NpServer {
	return MakeReplicatedNpServer(npc, address, false, nil)
}

// TODO: establish connections with other servers.
func MakeReplicatedNpServer(npc NpConn, address string, replicated bool, config *NpServerReplConfig) *NpServer {
	if replicated {
		log.Printf("Starting replicated server: %v", config)
	}
	srv := &NpServer{npc, "", replicated, config}
	var l net.Listener
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	srv.addr = l.Addr().String()
	if replicated {
		srv.connectToReplicas()
	}
	db.DLPrintf("9PCHAN", "listen %v  myaddr %v\n", address, srv.addr)
	go srv.runsrv(l)
	return srv
}

func (srv *NpServer) connectToReplicas() {
	srv.replConfig.HeadChan = srv.replConfig.MakeNpChan(srv.replConfig.HeadAddr)
	srv.replConfig.TailChan = srv.replConfig.MakeNpChan(srv.replConfig.TailAddr)
	srv.replConfig.PrevChan = srv.replConfig.MakeNpChan(srv.replConfig.PrevAddr)
	srv.replConfig.NextChan = srv.replConfig.MakeNpChan(srv.replConfig.NextAddr)
}

func (srv *NpServer) MyAddr() string {
	return srv.addr
}

func (srv *NpServer) runsrv(l net.Listener) {
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}

		// If we aren't replicated or we're at the end of the chain, create a normal
		// channel.
		if !srv.replicated || srv.isTail() {
			MakeChannel(srv.npc, conn)
		} else {
			// Else, make a relay channel which forwards calls along the chain.
			log.Printf("Relay chan from %v -> %v", conn.RemoteAddr(), srv.addr)
			MakeRelayChannel(srv.npc, conn, srv.replConfig.NextChan)
		}
		//		if srv.replicated && srv.isChainConn(conn.RemoteAddr()) {
		//			// TODO: make a special channel.
		//		} else {
		//			// TODO: clean up
		//			if srv.replicated {
		//				//        if srv.isHead()
		//			} else {
		//				MakeChannel(srv.npc, conn)
		//			}
		//		}
	}
}

func (srv *NpServer) isHead() bool {
	return srv.addr == srv.replConfig.HeadAddr
}

func (srv *NpServer) isTail() bool {
	return srv.addr == srv.replConfig.TailAddr
}

func (srv *NpServer) isChainConn(addr net.Addr) bool {
	return addr.String() == srv.replConfig.PrevAddr
}

func (srv *NpServer) String() string {
	return fmt.Sprintf("{ addr: %v replicated: %v config: %v }", srv.addr, srv.replicated, srv.replConfig)
}

func (c *NpServerReplConfig) String() string {
	return fmt.Sprintf("{ head: %v tail: %v prev: %v next: %v}", c.HeadAddr, c.TailAddr, c.PrevAddr, c.NextAddr)
}
