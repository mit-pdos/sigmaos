package srv

import (
	"fmt"
	"net"

	db "sigmaos/debug"
	"sigmaos/util/io/demux"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type NewConnI interface {
	NewConn(*sp.Tprincipal, net.Conn) *demux.DemuxSrv
}

type NetServer struct {
	pe      *proc.ProcEnv
	npc     *dialproxyclnt.DialProxyClnt
	ep      *sp.Tendpoint
	l       *dialproxyclnt.Listener
	newConn NewConnI
}

func NewNetServer(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, addr *sp.Taddr, newConn NewConnI) *NetServer {
	return NewNetServerEPType(pe, npc, addr, sp.INTERNAL_EP, newConn)
}

// Special-case used just for proxy
func NewNetServerEPType(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, addr *sp.Taddr, eptype sp.TTendpoint, newConn NewConnI) *NetServer {
	srv := &NetServer{
		pe:      pe,
		newConn: newConn,
		npc:     npc,
	}
	db.DPrintf(db.NETSRV, "Listen addr %v", addr.IPPort())
	// Create and start the main server listener
	ep, l, err := npc.Listen(eptype, addr)
	if err != nil {
		db.DFatalf("Listen error: %v", err)
	}
	srv.ep = ep
	srv.l = l
	db.DPrintf(db.NETSRV, "listen %v myaddr %v\n", addr, srv.ep)
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

func (srv *NetServer) runsrv(l *dialproxyclnt.Listener) {
	for {
		conn, p, err := l.AcceptGetPrincipal()
		db.DPrintf(db.NETSRV, "Accept conn from principal %v", p)
		if err != nil {
			db.DPrintf(db.NETSRV, "%v: Accept err %v", srv.pe.GetPID(), err)
			return
		}
		db.DPrintf(db.NETSRV, "accept %v %v\n", l, conn)
		srv.newConn.NewConn(p, conn)
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ ep: %v }", srv.ep)
}
