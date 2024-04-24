package netsrv

import (
	"fmt"
	"net"

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
	pe      *proc.ProcEnv
	npc     *netsigma.NetProxyClnt
	mnt     *sp.Tendpoint
	l       net.Listener
	newConn NewConnI
}

func NewNetServer(pe *proc.ProcEnv, npc *netsigma.NetProxyClnt, addr *sp.Taddr, newConn NewConnI) *NetServer {
	srv := &NetServer{
		pe:      pe,
		newConn: newConn,
		npc:     npc,
	}
	db.DPrintf(db.PORT, "Listen addr %v", addr.IPPort())
	// Create and start the main server listener
	mnt, l, err := npc.Listen(addr)
	if err != nil {
		db.DFatalf("Listen error: %v", err)
	}
	srv.mnt = mnt
	srv.mnt = mnt
	srv.l = l
	db.DPrintf(db.PORT, "listen %v myaddr %v\n", addr, srv.mnt)
	go srv.runsrv(l)
	return srv
}

func (srv *NetServer) GetEndpoint() *sp.Tendpoint {
	return srv.mnt
}

func (srv *NetServer) CloseListener() error {
	db.DPrintf(db.NETSRV, "Close %v\n", srv.mnt)
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
	return fmt.Sprintf("{ mnt: %v }", srv.mnt)
}
