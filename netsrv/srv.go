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
