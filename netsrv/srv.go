package netsrv

import (
	"fmt"
	"log"
	"net"

	db "ulambda/debug"
	"ulambda/npapi"
)

type NetServer struct {
	addr       string
	fssrv      npapi.FsServer
	wireCompat bool
	replicated bool
	replyCache *ReplyCache
	replConfig *NetServerReplConfig
}

func MakeNetServer(address string, fssrv npapi.FsServer) *NetServer {
	return MakeReplicatedNetServer(fssrv, address, false, false, "", nil)
}

func MakeNpServerWireCompatible(address string, fssrv npapi.FsServer) *NetServer {
	return MakeReplicatedNetServer(fssrv, address, true, false, "", nil)
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

		// If we aren't replicated or we're at the end of the chain, create a normal
		// channel.
		if !srv.replicated {
			MakeSrvConn(conn, srv.fssrv, srv.wireCompat)
		} else {
			// Else, make a relay channel which forwards calls along the chain.
			db.DLPrintf("9PCHAN", "relay chan from %v -> %v\n", conn.RemoteAddr(), l.Addr())
			srv.MakeRelayConn(srv.fssrv, conn, srv.replConfig.ops, srv.replConfig.fids)
		}
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ addr: %v replicated: %v config: %v }", srv.addr, srv.replicated, srv.replConfig)
}
