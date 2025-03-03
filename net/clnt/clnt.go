// The netclnt package establishes a TCP connection to a server.
package clnt

import (
	"net"
	"sync"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type NetClnt struct {
	mu    sync.Mutex
	pe    *proc.ProcEnv
	npc   *dialproxyclnt.DialProxyClnt
	ep    *sp.Tendpoint
	addr  *sp.Taddr
	realm sp.Trealm
}

func NewNetClnt(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt, ep *sp.Tendpoint) (net.Conn, *serr.Err) {
	db.DPrintf(db.NETCLNT, "NewNetClnt to %v\n", ep)
	nc := &NetClnt{
		pe:  pe,
		npc: npc,
		ep:  ep,
	}
	conn, err := nc.connect(ep)
	db.DPrintf(db.NETCLNT_ERR, "NewNetClnt connect %v err %v\n", ep, err)
	return conn, err
}

func (nc *NetClnt) connect(ep *sp.Tendpoint) (net.Conn, *serr.Err) {
	db.DPrintf(db.NETCLNT, "NetClnt connect to any of %v, starting w. %v\n", ep, ep.Addrs()[0])
	for i, addr := range ep.Addrs() {
		if i > 0 {
			ep.Addr = append(ep.Addr[1:], ep.Addr[0])
		}
		c, err := nc.npc.Dial(ep)
		db.DPrintf(db.NETCLNT, "Dial %v addr.Addr %v\n", addr.IPPort(), err)
		if err != nil {
			continue
		}
		db.DPrintf(db.NETCLNT, "NetClnt connected %v -> %v\n", c.LocalAddr(), addr)
		return c, nil
	}
	db.DPrintf(db.NETCLNT_ERR, "NetClnt unable to connect to any of %v\n", ep)
	return nil, serr.NewErr(serr.TErrUnreachable, "no connection")
}
