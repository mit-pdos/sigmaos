package netproxy

import (
	"net"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sync/atomic"
)

type Tlid uint64
type Tlidctr = atomic.Uint64

type Listener struct {
	npc *NetProxyClnt
	lid Tlid
	la  *ListenerAddr
}

type ListenerAddr struct {
	ep *sp.Tendpoint
}

func NewListener(npc *NetProxyClnt, lid Tlid, ep *sp.Tendpoint) net.Listener {
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
	db.DFatalf("Unimplemented")
	return nil
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
