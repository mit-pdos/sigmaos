// The netclnt package establishes a TCP connection to a server.
package netclnt

import (
	"net"
	"sync"

	// "time"

	db "sigmaos/debug"
	"sigmaos/netproxy"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type NetClnt struct {
	mu     sync.Mutex
	pe     *proc.ProcEnv
	npc    *netproxy.NetProxyClnt
	conn   net.Conn
	ep     *sp.Tendpoint
	addr   *sp.Taddr
	closed bool
	realm  sp.Trealm
}

func NewNetClnt(pe *proc.ProcEnv, npc *netproxy.NetProxyClnt, ep *sp.Tendpoint) (*NetClnt, *serr.Err) {
	db.DPrintf(db.NETCLNT, "NewNetClnt to %v\n", ep)
	nc := &NetClnt{
		pe:  pe,
		npc: npc,
		ep:  ep,
	}
	if err := nc.connect(ep); err != nil {
		db.DPrintf(db.NETCLNT_ERR, "NewNetClnt connect %v err %v\n", ep, err)
		return nil, err
	}
	return nc, nil
}

func (nc *NetClnt) Conn() net.Conn {
	return nc.conn
}

func (nc *NetClnt) Dst() string {
	return nc.conn.RemoteAddr().String()
}

func (nc *NetClnt) Src() string {
	return nc.conn.LocalAddr().String()
}

func (nc *NetClnt) Close() error {
	return nc.conn.Close()
}

func (nc *NetClnt) connect(ep *sp.Tendpoint) *serr.Err {
	if !nc.pe.GetVerifyEndpoints() && len(ep.Claims.Addr) > 0 {
		ep.Claims.Addr = netsigma.Rearrange(nc.pe.GetNet(), ep.Claims.Addr)
	}
	db.DPrintf(db.PORT, "NetClnt %v connect to any of %v, starting w. %v\n", nc.pe.GetNet(), ep, ep.Addrs()[0])
	//	for _, addr := range addrs {
	for i, addr := range ep.Addrs() {
		if i > 0 {
			if nc.pe.GetVerifyEndpoints() {
				// TODO XXX: support multi-dialing
				db.DFatalf("Do not support multi-dialing yet: %v", ep.Addrs())
			}
			ep.Claims.Addr = append(ep.Claims.Addr[1:], ep.Claims.Addr[0])
		}
		c, err := nc.npc.Dial(ep)
		db.DPrintf(db.PORT, "Dial %v addr.Addr %v\n", addr.IPPort(), err)
		if err != nil {
			continue
		}
		nc.conn = c
		nc.addr = addr
		db.DPrintf(db.PORT, "NetClnt connected %v -> %v\n", c.LocalAddr(), nc.addr)
		return nil
	}
	db.DPrintf(db.NETCLNT_ERR, "NetClnt unable to connect to any of %v\n", ep)
	return serr.NewErr(serr.TErrUnreachable, "no connection")
}
