package netsigma

import (
	"net"
	"sync"

	db "sigmaos/debug"
	"sigmaos/netsigma/proto"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
)

type NetProxyClnt struct {
	sync.Mutex
	pe           *proc.ProcEnv
	directDialFn DialFn
	proxyDialFn  DialFn
	rpcc         *rpcclnt.RPCClnt
	rpcch        *NetProxyRPCCh
}

func NewNetProxyClnt(pe *proc.ProcEnv) *NetProxyClnt {
	return &NetProxyClnt{
		pe:           pe,
		directDialFn: DialDirect,
	}
}

func (npc *NetProxyClnt) Dial(addr *sp.Taddr) (net.Conn, error) {
	var c net.Conn
	var err error
	// TODO XXX : a hack to force use by sigmaclntd procs. Remove GetPriv
	if npc.pe.GetUseSigmaclntd() || !npc.pe.GetPrivileged() {
		db.DPrintf(db.NETPROXYCLNT, "proxyDial %v", addr)
		c, err = npc.proxyDial(addr)
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directDial %v", addr)
		c, err = npc.directDialFn(addr)
	}
	return c, err
}

// Lazily init connection to the netproxy srv
func (npc *NetProxyClnt) init() error {
	// Connect to the netproxy server
	ch, err := NewNetProxyRPCCh()
	if err != nil {
		return err
	}
	npc.rpcch = ch
	npc.rpcc = rpcclnt.NewRPCClnt(ch)
	return nil
}

func (npc *NetProxyClnt) proxyDial(addr *sp.Taddr) (net.Conn, error) {
	npc.Lock()
	defer npc.Unlock()

	// Ensure that the connection to the netproxy server has been initialized
	if npc.rpcc == nil {
		if err := npc.init(); err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error dial netproxysrv %v", err)
			return nil, err
		}
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyDial request addr %v", npc.rpcch.conn, addr.String())
	req := &proto.DialRequest{
		Addr: addr,
	}
	res := &proto.DialResponse{}
	if err := npc.rpcc.RPC("NetProxySrv.Dial", req, res); err != nil {
		return nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "proxyDial response %v", res)
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error Dial: %v", err)
		return nil, err
	}
	return npc.rpcch.GetReturnedConn()
}
