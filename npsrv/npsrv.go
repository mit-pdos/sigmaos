package npsrv

import (
	"log"
	"net"

	db "ulambda/debug"
)

type NpConn interface {
	Connect(net.Conn) NpAPI
}

type NpServer struct {
	npc  NpConn
	addr string
}

func MakeNpServer(npc NpConn, server string, debug bool) *NpServer {
	db.SetDebug(debug)
	srv := &NpServer{npc, ""}
	var l net.Listener
	l, err := net.Listen("tcp", server)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	srv.addr = l.Addr().String()
	db.DPrintf("myaddr %v\n", srv.addr)
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
		fsconn := MakeChannel(srv.npc, conn)
		go fsconn.Serve()
	}
}
