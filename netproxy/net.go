package netproxy

import (
	"fmt"
	"net"
	"sync/atomic"

	"sigmaos/auth"
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
		db.DPrintf(db.NETSIGMA_ERR, "Dial direct addr err %v: err %v", ep.Addrs()[0], err)
	} else {
		db.DPrintf(db.NETSIGMA, "Dial direct addr ok %v", ep.Addrs()[0])
		if ep.Type() == sp.INTERNAL_EP {
			if err := writeConnPreamble(c, p); err != nil {
				db.DPrintf(db.NETSIGMA_ERR, "Write preamble err: %v", err)
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

func AcceptDirect(l net.Listener, withPreamble bool) (net.Conn, *sp.Tprincipal, error) {
	p := sp.NoPrincipal()
	c, err := l.Accept()
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Accept on %v err: %v", l.Addr(), err)
	} else {
		if withPreamble {
			var err error
			p, err = readConnPreamble(c)
			if err != nil {
				db.DPrintf(db.NETSIGMA_ERR, "Write preamble err: %v", err)
				return nil, nil, err
			}
		}
		db.DPrintf(db.NETSIGMA, "Accept on %v ok p %v local addr: %v", l.Addr(), p, c.LocalAddr())
	}
	return c, p, err
}

func NewEndpoint(verifyEndpoints bool, amgr auth.AuthMgr, ip sp.Tip, realm sp.Trealm, l net.Listener) (*sp.Tendpoint, error) {
	host, port, err := netsigma.QualifyAddrLocalIP(ip, l.Addr().String())
	if err != nil {
		db.DPrintf(db.ERROR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		return nil, err
	}
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{sp.NewTaddrRealm(host, sp.INNER_CONTAINER_IP, port, realm.String())}, realm)
	if verifyEndpoints && amgr == nil {
		db.DFatalf("Error construct endpoint without AuthMgr")
		return nil, fmt.Errorf("Try to construct endpoint without authsrv")
	}
	if amgr != nil {
		// Sign the endpoint
		if err := amgr.MintAndSetEndpointToken(ep); err != nil {
			db.DFatalf("Error sign endpoint: %v", err)
			return nil, err
		}
	}
	return ep, nil
}
