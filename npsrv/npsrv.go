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
	npc        NpConn
	addr       string
	replicated bool
	replConfig *NpServerReplConfig
}

type NpServerReplConfig struct {
	HeadAddr string
	TailAddr string
	NextAddr string
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
	db.DLPrintf("9PCHAN", "listen %v  myaddr %v\n", address, srv.addr)
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
		MakeChannel(srv.npc, conn)
	}
}
