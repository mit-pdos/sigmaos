package netsrv

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type MarshalF func(*sessp.FcallMsg, *bufio.Writer) *serr.Err
type UnmarshalF func(rdr io.Reader) (*sessp.FcallMsg, *serr.Err)

type NetServer struct {
	addr      string
	sesssrv   sp.SessServer
	marshal   MarshalF
	unmarshal UnmarshalF
}

func MakeNetServer(ss sp.SessServer, address string, m MarshalF, u UnmarshalF) *NetServer {
	srv := &NetServer{"",
		ss,
		m,
		u,
	}

	// Create and start the main server listener
	var l net.Listener
	l, err := net.Listen("tcp", address)
	if err != nil {
		db.DFatalf("Listen error: %v", err)
	}
	a, err := container.QualifyAddr(l.Addr().String())
	if err != nil {
		db.DFatalf("QualifyAddr %v error: %v", a, err)
	}
	srv.addr = a
	db.DPrintf(db.PORT, "listen %v myaddr %v\n", address, a)
	go srv.runsrv(l)
	return srv
}

func (srv *NetServer) MyAddr() string {
	return srv.addr
}

func (srv *NetServer) runsrv(l net.Listener) {
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DFatalf("%v: Accept error: %v", proc.GetName(), err)
		}
		db.DPrintf(db.NETSRV, "accept %v %v\n", l, conn)
		MakeSrvConn(srv, conn)
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ addr: %v }", srv.addr)
}
