package netsrv

import (
	"fmt"
	"net"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netproxy"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type NewConnI interface {
	NewConn(net.Conn) *demux.DemuxSrv
}

type NetServer struct {
	pe      *proc.ProcEnv
	npc     *netproxy.NetProxyClnt
	ep      *sp.Tendpoint
	l       net.Listener
	newConn NewConnI
}

func NewNetServer(pe *proc.ProcEnv, npc *netproxy.NetProxyClnt, addr *sp.Taddr, newConn NewConnI) *NetServer {
	srv := &NetServer{
		pe:      pe,
		newConn: newConn,
		npc:     npc,
	}
	db.DPrintf(db.PORT, "Listen addr %v", addr.IPPort())
	// Create and start the main server listener
	ep, l, err := npc.Listen(addr)
	if err != nil {
		db.DFatalf("Listen error: %v", err)
	}
	srv.ep = ep
	srv.ep = ep
	srv.l = l
	db.DPrintf(db.PORT, "listen %v myaddr %v\n", addr, srv.ep)
	go srv.runsrv(l)
	return srv
}

func (srv *NetServer) GetEndpoint() *sp.Tendpoint {
	return srv.ep
}

func (srv *NetServer) CloseListener() error {
	db.DPrintf(db.NETSRV, "Close %v\n", srv.ep)
	return srv.l.Close()
}

func (srv *NetServer) runsrv(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DPrintf(db.NETSRV, "%v: Accept err %v", srv.pe.GetPID(), err)
			return
		}
		db.DPrintf(db.NETSRV, "accept %v %v\n", l, conn)
		srv.newConn.NewConn(conn)
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ ep: %v }", srv.ep)
}
