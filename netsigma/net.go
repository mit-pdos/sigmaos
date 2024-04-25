package netsigma

import (
	"net"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type DialFn func(ep *sp.Tendpoint) (net.Conn, error)
type ListenFn func(addr *sp.Taddr) (net.Listener, error)

func DialDirect(ep *sp.Tendpoint) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", ep.Addrs()[0].IPPort(), sp.Conf.Session.TIMEOUT/10)
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Dial direct addr err %v: err %v", ep.Addrs()[0], err)
	} else {
		db.DPrintf(db.NETSIGMA, "Dial direct addr ok %v", ep.Addrs()[0])
	}
	return c, err
}

func ListenDirect(addr *sp.Taddr) (net.Listener, error) {
	l, err := net.Listen("tcp", addr.IPPort())
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Listen on addr %v: err %v", addr, err)
	}
	db.DPrintf(db.NETSIGMA, "Listen on addr %v res addr %v", addr, l.Addr())
	return l, err
}
