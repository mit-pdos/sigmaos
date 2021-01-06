package npsrv

import (
	"log"
	"net"
)

type NpConn interface {
	Connect(net.Conn) NpAPI
}

type NpServer struct {
	npd  NpConn
	addr string
}

func MakeNpServer(npd NpConn, server string) *NpServer {
	srv := &NpServer{npd, ""}
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
		fsconn := MakeChannel(srv.npd, conn)
		go fsconn.Serve()
	}
}
