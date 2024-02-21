package netsrv

import (
	"fmt"
	"net"

	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netsigma"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type NewConnI interface {
	NewConn(net.Conn) *demux.DemuxSrv
}

type NetServer struct {
	pcfg    *proc.ProcEnv
	addr    *sp.Taddr
	l       net.Listener
	newConn NewConnI
}

func NewNetServer(pcfg *proc.ProcEnv, addr *sp.Taddr, newConn NewConnI) *NetServer {
	srv := &NetServer{pcfg: pcfg, newConn: newConn}
	db.DPrintf(db.PORT, "Listen addr %v", addr.IPPort())
	// Create and start the main server listener
	var l net.Listener
	l, err := net.Listen("tcp", addr.IPPort())
	if err != nil {
		db.DFatalf("Listen error: %v", err)
	}
	h, p, err := netsigma.QualifyAddrLocalIP(pcfg.GetInnerContainerIP(), l.Addr().String())
	if err != nil {
		db.DFatalf("QualifyAddr \"%v\" -> \"%v:%v\" error: %v\n%s", l.Addr().String(), h, p, err, debug.Stack())
	}
	srv.addr = sp.NewTaddrRealm(h, sp.INNER_CONTAINER_IP, p, pcfg.GetNet())
	srv.l = l
	db.DPrintf(db.PORT, "listen %v myaddr %v\n", addr, srv.addr)
	go srv.runsrv(l)
	return srv
}

func (srv *NetServer) MyAddr() *sp.Taddr {
	return srv.addr
}

func (srv *NetServer) CloseListener() error {
	db.DPrintf(db.NETSRV, "Close %v\n", srv.addr)
	return srv.l.Close()
}

func (srv *NetServer) runsrv(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DPrintf(db.NETSRV, "%v: Accept err %v", srv.pcfg.GetPID(), err)
			return
		}
		db.DPrintf(db.NETSRV, "accept %v %v\n", l, conn)
		srv.newConn.NewConn(conn)
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ addr: %v }", srv.addr)
}
