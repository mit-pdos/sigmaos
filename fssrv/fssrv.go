package fssrv

import (
	"log"
	"net"
	"net/rpc"
)

type FsClient interface{}

type Fsd interface {
	Connect(net.Conn) FsClient
}

type FsServer struct {
	fsd  Fsd
	addr string
}

func MakeFsServer(fsd Fsd, server string) *FsServer {
	srv := &FsServer{fsd, ""}
	var l net.Listener
	l, err := net.Listen("tcp", server)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	srv.addr = l.Addr().String()
	log.Printf("myaddr %v\n", srv.addr)
	go srv.runsrv(l)
	return srv
}

func (srv *FsServer) MyAddr() string {
	return srv.addr
}

func (srv *FsServer) runsrv(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}
		clnt := MakeFsConn(srv.fsd, conn)
		rpc.Register(clnt)
		go rpc.ServeConn(clnt.conn)
	}
}
