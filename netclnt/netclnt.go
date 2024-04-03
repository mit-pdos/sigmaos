// The netclnt package establishes a TCP connection to a server.
package netclnt

import (
	"net"
	"sync"

	// "time"

	db "sigmaos/debug"
	"sigmaos/netsigma"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type NetClnt struct {
	mu     sync.Mutex
	conn   net.Conn
	addr   *sp.Taddr
	closed bool
	realm  sp.Trealm
}

func NewNetClnt(clntnet string, addrs sp.Taddrs) (*NetClnt, *serr.Err) {
	db.DPrintf(db.NETCLNT, "NewNetClnt to %v\n", addrs)
	nc := &NetClnt{}
	if err := nc.connect(clntnet, addrs); err != nil {
		db.DPrintf(db.NETCLNT_ERR, "NewNetClnt connect %v err %v\n", addrs, err)
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

func (nc *NetClnt) connect(clntnet string, addrs sp.Taddrs) *serr.Err {
	addrs = netsigma.Rearrange(clntnet, addrs)
	db.DPrintf(db.PORT, "NetClnt %v connect to any of %v, starting w. %v\n", clntnet, addrs, addrs[0])
	for _, addr := range addrs {
		tcpaddr, err := net.ResolveTCPAddr("tcp", addr.IPPort())
		if err != nil {
			continue
		}
		//c, err := net.DialTimeout("tcp", addr.IPPort(), sp.Conf.Session.TIMEOUT/10)
		c, err := net.DialTCP("tcp", nil, tcpaddr)
		db.DPrintf(db.PORT, "Dial %v addr.Addr %v\n", addr.IPPort(), err)
		if err != nil {
			continue
		}
		c.SetNoDelay(true)
		nc.conn = c
		nc.addr = addr
		db.DPrintf(db.PORT, "NetClnt connected %v -> %v\n", c.LocalAddr(), nc.addr)
		return nil
	}
	db.DPrintf(db.NETCLNT_ERR, "NetClnt unable to connect to any of %v\n", addrs)
	return serr.NewErr(serr.TErrUnreachable, "no connection")
}
