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
	pe             *proc.ProcEnv
	directDialFn   DialFn
	directListenFn ListenFn
	rpcc           *rpcclnt.RPCClnt
	rpcch          *NetProxyRPCCh
}

func NewNetProxyClnt(pe *proc.ProcEnv) *NetProxyClnt {
	return &NetProxyClnt{
		pe:             pe,
		directDialFn:   DialDirect,
		directListenFn: ListenDirect,
	}
}

func (npc *NetProxyClnt) Dial(mnt *sp.Tmount) (net.Conn, error) {
	var c net.Conn
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyDial %v", mnt)
		c, err = npc.proxyDial(mnt)
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directDial %v", mnt)
		c, err = npc.directDialFn(mnt)
	}
	return c, err
}

func (npc *NetProxyClnt) Listen(addr *sp.Taddr) (net.Listener, error) {
	var l net.Listener
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyListen %v", addr)
		l, err = npc.proxyListen(addr)
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directListen %v", addr)
		l, err = npc.directListenFn(addr)
	}
	return l, err
}

func (npc *NetProxyClnt) useProxy() bool {
	// TODO XXX : a hack to force use by sigmaclntd procs. Remove GetPriv part
	return npc.pe.GetUseSigmaclntd() || !npc.pe.GetPrivileged()
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

func (npc *NetProxyClnt) proxyDial(mnt *sp.Tmount) (net.Conn, error) {
	npc.Lock()
	defer npc.Unlock()

	// Ensure that the connection to the netproxy server has been initialized
	if npc.rpcc == nil {
		if err := npc.init(); err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error dial netproxysrv %v", err)
			return nil, err
		}
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyDial request mnt %v", npc.rpcch.conn, mnt)
	req := &proto.DialRequest{
		Mount: mnt.GetProto(),
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

func (npc *NetProxyClnt) proxyListen(addr *sp.Taddr) (net.Listener, error) {
	npc.Lock()
	defer npc.Unlock()

	// Ensure that the connection to the netproxy server has been initialized
	if npc.rpcc == nil {
		if err := npc.init(); err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error dial netproxysrv %v", err)
			return nil, err
		}
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyListen request addr", npc.rpcch.conn)
	req := &proto.ListenRequest{
		Addr: addr,
	}
	res := &proto.ListenResponse{}
	if err := npc.rpcc.RPC("NetProxySrv.Listen", req, res); err != nil {
		return nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "proxyListen response %v", res)
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error Listen: %v", err)
		return nil, err
	}
	return npc.rpcch.GetReturnedListener()
}
