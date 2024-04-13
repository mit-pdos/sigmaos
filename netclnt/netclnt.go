// The netclnt package establishes a TCP connection to a server.
package netclnt

import (
	"net"
	"sync"

	// "time"

	db "sigmaos/debug"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type NetClnt struct {
	mu     sync.Mutex
	pe     *proc.ProcEnv
	npc    *netsigma.NetProxyClnt
	conn   net.Conn
	mnt    *sp.Tmount
	addr   *sp.Taddr
	closed bool
	realm  sp.Trealm
}

func NewNetClnt(pe *proc.ProcEnv, npc *netsigma.NetProxyClnt, mnt *sp.Tmount) (*NetClnt, *serr.Err) {
	db.DPrintf(db.NETCLNT, "NewNetClnt to %v\n", mnt)
	nc := &NetClnt{
		pe:  pe,
		npc: npc,
		mnt: mnt,
	}
	if err := nc.connect(mnt); err != nil {
		db.DPrintf(db.NETCLNT_ERR, "NewNetClnt connect %v err %v\n", mnt, err)
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

func (nc *NetClnt) connect(mnt *sp.Tmount) *serr.Err {
	if !nc.pe.GetVerifyMounts() && len(mnt.Claims.Addr) > 0 {
		mnt.Claims.Addr = netsigma.Rearrange(nc.pe.GetNet(), mnt.Claims.Addr)
	}
	db.DPrintf(db.PORT, "NetClnt %v connect to any of %v, starting w. %v\n", nc.pe.GetNet(), mnt, mnt.Addrs()[0])
	//	for _, addr := range addrs {
	for i, addr := range mnt.Addrs() {
		if i > 0 {
			if nc.pe.GetVerifyMounts() {
				// TODO XXX: support multi-dialing
				db.DFatalf("Do not support multi-dialing yet: %v", mnt.Addrs())
			}
			mnt.Claims.Addr = append(mnt.Claims.Addr[1:], mnt.Claims.Addr[0])
		}
		c, err := nc.npc.Dial(mnt)
		db.DPrintf(db.PORT, "Dial %v addr.Addr %v\n", addr.IPPort(), err)
		if err != nil {
			continue
		}
		nc.conn = c
		nc.addr = addr
		db.DPrintf(db.PORT, "NetClnt connected %v -> %v\n", c.LocalAddr(), nc.addr)
		return nil
	}
	db.DPrintf(db.NETCLNT_ERR, "NetClnt unable to connect to any of %v\n", mnt)
	return serr.NewErr(serr.TErrUnreachable, "no connection")
}
