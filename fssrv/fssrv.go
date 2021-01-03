package fssrv

import (
	"log"
	"net"
	"net/rpc"
)

type Fsd interface {
	Connect(net.Conn) FsConn
}

type Conn struct {
	fsd  Fsd
	conn net.Conn
	clnt FsConn
}

func MakeConn(fsd Fsd, conn net.Conn) *Conn {
	clnt := fsd.Connect(conn)
	c := &Conn{fsd, conn, clnt}
	return c
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
		fsconn := MakeConn(srv.fsd, conn)
		rpc.Register(fsconn.clnt)
		go rpc.ServeConn(fsconn.conn)
	}
}
