package netproxy

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/netsigma"
	sp "sigmaos/sigmap"
)

const (
	INIT_MSG = "HELLO"
)

type Tlid uint64
type Tlidctr = atomic.Uint64

type DialFn func(ep *sp.Tendpoint) (net.Conn, error)
type ListenFn func(addr *sp.Taddr) (net.Listener, error)

func DialDirect(ep *sp.Tendpoint) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", ep.Addrs()[0].IPPort(), sp.Conf.Session.TIMEOUT/10)
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Dial direct addr err %v: err %v", ep.Addrs()[0], err)
	} else {
		db.DPrintf(db.NETSIGMA, "Dial direct addr ok %v", ep.Addrs()[0])
		if ep.Type() == sp.INTERNAL_EP {
			// TODO: send principal
			n, err := c.Write([]byte(INIT_MSG))
			if err != nil || n != len(INIT_MSG) {
				db.DPrintf(db.NETSIGMA_ERR, "Dial direct addr err %v: err %v", ep.Addrs()[0], err)
				return nil, fmt.Errorf("Error send principal preamble: n %v err %v", n, err)
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

func AcceptDirect(l net.Listener) (net.Conn, error) {
	c, err := l.Accept()
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Accept on %v err: %v", l.Addr(), err)
	} else {
		db.DPrintf(db.NETSIGMA, "Accept on %v ok local addr: %v", l.Addr(), c.LocalAddr())
		// TODO: optionally, don't read principal
		if true {
			// TODO: read principal
			b := make([]byte, len(INIT_MSG))
			n, err := io.ReadFull(c, b)
			if err != nil || n != len(INIT_MSG) {
				if string(b) != INIT_MSG {
					db.DFatalf("Unexpected init message: %v", string(b))
				}
				return nil, fmt.Errorf("Error read principal preamble: n %v err %v", n, err)
			}
		}
	}
	return c, err
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
