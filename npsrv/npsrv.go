package npsrv

import (
	"fmt"
	"log"
	"net"

	db "ulambda/debug"
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

func MakeNpServer(npc NpConn, address string) *NpServer {
	return MakeReplicatedNpServer(npc, address, false, "", nil)
}

func (srv *NpServer) MyAddr() string {
	return srv.addr
}

func (srv *NpServer) runsrv(l net.Listener, wrapped bool) {
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}

		// If we aren't replicated or we're at the end of the chain, create a normal
		// channel.
		if !srv.replicated {
			MakeChannel(srv.npc, conn)
		} else {
			// Else, make a relay channel which forwards calls along the chain.
			db.DLPrintf("9PCHAN", "relay chan from %v -> %v\n", conn.RemoteAddr(), l.Addr())
			MakeRelayChannel(srv.npc, conn, srv.replConfig.ops, wrapped)
		}
	}
}

func (srv *NpServer) String() string {
	return fmt.Sprintf("{ addr: %v replicated: %v config: %v }", srv.addr, srv.replicated, srv.replConfig)
}
