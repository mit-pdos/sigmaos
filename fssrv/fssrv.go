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
	fsd Fsd
}

func MakeFsServer(fsd Fsd, server string) *FsServer {
	srv := &FsServer{fsd}
	var l net.Listener
	l, err := net.Listen("tcp", server)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	addr := l.Addr()
	log.Printf("myaddr %v\n", addr)
	go srv.runsrv(l)
	return srv
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
