package netproxyclnt

import (
	"net"

	"sigmaos/netproxy"
	sp "sigmaos/sigmap"
)

// Must respect standard net.Listener API for compatibility
type Listener struct {
	npc *NetProxyClnt
	lid netproxy.Tlid
	la  *ListenerAddr
}

type ListenerAddr struct {
	ep *sp.Tendpoint
}

func NewListener(npc *NetProxyClnt, lid netproxy.Tlid, ep *sp.Tendpoint) *Listener {
	return &Listener{
		npc: npc,
		lid: lid,
		la:  NewListenerAddr(ep),
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	c, _, err := l.npc.Accept(l.lid)
	return c, err
}

func (l *Listener) AcceptGetPrincipal() (net.Conn, *sp.Tprincipal, error) {
	return l.npc.Accept(l.lid)
}

func (l *Listener) Close() error {
	return l.npc.Close(l.lid)
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
