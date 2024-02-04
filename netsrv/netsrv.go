package netsrv

import (
	"fmt"
	"net"

	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/netsigma"
	"sigmaos/proc"
	//"sigmaos/serr"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type NetServer struct {
	pcfg    *proc.ProcEnv
	addr    *sp.Taddr
	sesssrv sps.SessServer
	l       net.Listener
	nframe  int
}

func NewNetServer(pcfg *proc.ProcEnv, sesssrv sps.SessServer, addr *sp.Taddr, nframe int) *NetServer {
	srv := &NetServer{pcfg: pcfg, sesssrv: sesssrv, nframe: nframe}

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
	db.DPrintf(db.ALWAYS, "Close %v\n", srv.addr)
	return srv.l.Close()
}

func (srv *NetServer) runsrv(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DPrintf(db.ALWAYS, "%v: Accept err %v", srv.pcfg.GetPID(), err)
			return
		}
		db.DPrintf(db.NETSRV, "accept %v %v\n", l, conn)
		NewNetSrvConn(srv, conn)
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ addr: %v }", srv.addr)
}
