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

// Returns once a connection has been accepted from an authorized principal, or
// there is an unexpected error
func AcceptFromAuthorizedPrincipal(l net.Listener, getPrincipal bool, isAuthorized func(*sp.Tprincipal) bool) (net.Conn, *sp.Tprincipal, error) {
	for {
		proxyConn, p, err := AcceptDirect(l, getPrincipal)
		if err != nil {
			// Report unexpected errors
			db.DPrintf(db.NETSIGMA_ERR, "Error accept direct: %v", err)
			return nil, nil, err
		}
		// For now, connections from the outside world are always allowed
		if getPrincipal {
			// If the client is not authorized to connect to the server,
			// close the connection, and retry the accept.
			if !isAuthorized(p) {
				db.DPrintf(db.NETSIGMA_ERR, "Attempted connection from unauthorized principal %v", p)
				proxyConn.Close()
				continue
			}
		}
		return proxyConn, p, err
	}
}

func NewEndpoint(ept sp.TTendpoint, ip sp.Tip, l net.Listener) (*sp.Tendpoint, error) {
	host, port, err := netsigma.QualifyAddrLocalIP(ip, l.Addr().String())
	if err != nil {
		db.DPrintf(db.ERROR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		return nil, err
	}
	return sp.NewEndpoint(ept, sp.Taddrs{sp.NewTaddrRealm(host, sp.INNER_CONTAINER_IP, port)}), nil
}

// Returns true if the client principal, cliP, is authorized to connect to the server principal, srvP
func ConnectionIsAuthorized(srvP *sp.Tprincipal, cliP *sp.Tprincipal) bool {
	// If server and client realms match, authorized
	if srvP.GetRealm() == cliP.GetRealm() {
		return true
	}
	// If the client belongs to the root realm, authorized
	if cliP.GetRealm() == sp.ROOTREALM {
		return true
	}
	// If the server belongs to the root realm, authorized
	if srvP.GetRealm() == sp.ROOTREALM {
		return true
	}
	// Unauthorized
	return false
}
