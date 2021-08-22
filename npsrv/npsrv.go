package npsrv

import (
	"fmt"
	"log"
	"net"

	db "ulambda/debug"
	"ulambda/npapi"
)

type NpServer struct {
	addr       string
	fssrv      npapi.FsServer
	wireCompat bool
	replicated bool
	replyCache *ReplyCache
	replConfig *NpServerReplConfig
}

func MakeNpServer(address string, fssrv npapi.FsServer) *NpServer {
	return MakeReplicatedNpServer(fssrv, address, false, false, "", nil)
}

func MakeNpServerWireCompatible(address string, fssrv npapi.FsServer) *NpServer {
	return MakeReplicatedNpServer(fssrv, address, true, false, "", nil)
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
		if !srv.replicated {
			MakeChannel(conn, srv.fssrv, srv.wireCompat)
		} else {
			// Else, make a relay channel which forwards calls along the chain.
			db.DLPrintf("9PCHAN", "relay chan from %v -> %v\n", conn.RemoteAddr(), l.Addr())
			srv.MakeRelayChannel(srv.fssrv, conn, srv.replConfig.ops, srv.replConfig.fids)
		}
	}
}

func (srv *NpServer) String() string {
	return fmt.Sprintf("{ addr: %v replicated: %v config: %v }", srv.addr, srv.replicated, srv.replConfig)
}
