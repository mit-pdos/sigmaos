package netproxyclnt

import (
	"net"

	"sigmaos/netproxy"
	sp "sigmaos/sigmap"
)

type Listener struct {
	npc *NetProxyClnt
	lid netproxy.Tlid
	la  *ListenerAddr
}

type ListenerAddr struct {
	ep *sp.Tendpoint
}

func NewListener(npc *NetProxyClnt, lid netproxy.Tlid, ep *sp.Tendpoint) net.Listener {
	return &Listener{
		npc: npc,
		lid: lid,
		la:  NewListenerAddr(ep),
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	return l.npc.proxyAccept(l.lid)
}

func (l *Listener) Close() error {
	return l.npc.proxyClose(l.lid)
}

func (l *Listener) Addr() net.Addr {
	return l.la
}

func NewListenerAddr(ep *sp.Tendpoint) *ListenerAddr {
	return &ListenerAddr{
		ep: ep,
	}
}

func (la *ListenerAddr) Network() string {
	return "tcp"
}

func (la *ListenerAddr) String() string {
	return la.ep.Addrs()[0].IPPort()
}
