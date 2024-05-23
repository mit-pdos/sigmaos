package netproxy

import (
	"net"
	"sync/atomic"

	db "sigmaos/debug"
	"sigmaos/netsigma"
	sp "sigmaos/sigmap"
)

type Tlid uint64
type Tlidctr = atomic.Uint64

type DialFn func(ep *sp.Tendpoint) (net.Conn, error)
type ListenFn func(addr *sp.Taddr) (net.Listener, error)

func DialDirect(p *sp.Tprincipal, ep *sp.Tendpoint) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", ep.Addrs()[0].IPPort(), sp.Conf.Session.TIMEOUT/10)
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "[%v] Dial direct addr err %v: err %v", p, ep.Addrs()[0], err)
	} else {
		db.DPrintf(db.NETSIGMA, "[%v] Dial direct addr ok %v", p, ep.Addrs()[0])
		if ep.Type() == sp.INTERNAL_EP {
			if err := writeConnPreamble(c, p); err != nil {
				db.DPrintf(db.NETSIGMA_ERR, "[%v] Write preamble err: %v", p, err)
				return nil, err
			}
		}
	}
	return c, err
}

func ListenDirect(addr *sp.Taddr) (net.Listener, error) {
	l, err := net.Listen("tcp", addr.IPPort())
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Listen on addr %v: err %v", addr, err)
	} else {
		db.DPrintf(db.NETSIGMA, "Listen on addr %v res addr %v", addr, l.Addr())
	}
	return l, err
}

func AcceptDirect(l net.Listener, getPrincipal bool) (net.Conn, *sp.Tprincipal, error) {
	p := sp.NoPrincipal()
	c, err := l.Accept()
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Accept on %v err: %v", l.Addr(), err)
	} else {
		if getPrincipal {
			var err error
			p, err = readConnPreamble(c)
			if err != nil {
				db.DPrintf(db.NETSIGMA_ERR, "Read preamble err: %v", err)
				return nil, nil, err
			}
		}
		db.DPrintf(db.NETSIGMA, "[%v] Accept on %v ok local addr: %v", p, l.Addr(), c.LocalAddr())
	}
	return c, p, err
}

func NewEndpoint(ept sp.TTendpoint, ip sp.Tip, realm sp.Trealm, l net.Listener) (*sp.Tendpoint, error) {
	host, port, err := netsigma.QualifyAddrLocalIP(ip, l.Addr().String())
	if err != nil {
		db.DPrintf(db.ERROR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		return nil, err
	}
	return sp.NewEndpoint(ept, sp.Taddrs{sp.NewTaddrRealm(host, sp.INNER_CONTAINER_IP, port, realm.String())}, realm), nil
}
