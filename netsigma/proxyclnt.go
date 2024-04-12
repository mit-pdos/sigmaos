package netsigma

import (
	"fmt"
	"net"
	"sync"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/netsigma/proto"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
)

type NetProxyClnt struct {
	sync.Mutex
	pe             *proc.ProcEnv
	canSignMounts  bool
	verifyMounts   bool
	auth           auth.AuthSrv
	directDialFn   DialFn
	directListenFn ListenFn
	rpcc           *rpcclnt.RPCClnt
	rpcch          *NetProxyRPCCh
}

func NewNetProxyClnt(pe *proc.ProcEnv, as auth.AuthSrv) *NetProxyClnt {
	return &NetProxyClnt{
		pe:             pe,
		canSignMounts:  as != nil,
		verifyMounts:   pe.GetVerifyMounts(),
		auth:           as,
		directDialFn:   DialDirect,
		directListenFn: ListenDirect,
	}
}

func (npc *NetProxyClnt) SetAuthSrv(as auth.AuthSrv) {
	npc.Lock()
	defer npc.Unlock()

	npc.auth = as
	npc.canSignMounts = true
}

func (npc *NetProxyClnt) GetAuthSrv() auth.AuthSrv {
	npc.Lock()
	defer npc.Unlock()

	return npc.auth
}

func (npc *NetProxyClnt) Dial(mnt *sp.Tmount) (net.Conn, error) {
	var c net.Conn
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyDial %v", mnt)
		c, err = npc.proxyDial(mnt)
		db.DPrintf(db.NETPROXYCLNT, "proxyDial %v done ok:%v", mnt, err == nil)
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directDial %v", mnt)
		c, err = npc.directDialFn(mnt)
		db.DPrintf(db.NETPROXYCLNT, "directDial done ok:%v", err == nil)
	}
	return c, err
}

func (npc *NetProxyClnt) Listen(addr *sp.Taddr) (*sp.Tmount, net.Listener, error) {
	var mnt *sp.Tmount
	var l net.Listener
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyListen %v", addr)
		mnt, l, err = npc.proxyListen(addr)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error proxyListen %v: %v", addr, err)
			return nil, nil, err
		}
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directListen %v", addr)
		if npc.verifyMounts && !npc.canSignMounts {
			err := fmt.Errorf("Try to listen on netproxyclnt without AuthSrv")
			db.DPrintf(db.ERROR, "Err listen: %v", err)
			return nil, nil, err
		}
		l, err = npc.directListenFn(addr)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error directListen %v: %v", addr, err)
			return nil, nil, err
		}
		mnt, err = constructMount(npc.verifyMounts, npc.auth, npc.pe.GetInnerContainerIP(), npc.pe.GetRealm(), l)
		if err != nil {
			db.DPrintf(db.ERROR, "Error construct mount: %v", err)
			return nil, nil, err
		}
	}
	return mnt, l, err
}

// If true, use the net proxy server for dialing & listening.
func (npc *NetProxyClnt) useProxy() bool {
	npc.Lock()
	defer npc.Unlock()

	return npc.pe.GetUseNetProxy()
}

// Lazily init connection to the netproxy srv
func (npc *NetProxyClnt) init() error {
	db.DPrintf(db.NETPROXYCLNT, "Init netproxyclnt %p", npc)
	// Connect to the netproxy server
	ch, err := NewNetProxyRPCCh(npc.pe)
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
	// Mounts should always have realms specified
	if mnt.GetRealm() == sp.NOT_SET {
		db.DPrintf(db.ERROR, "Dial mount without realm set: %v", mnt)
		return nil, fmt.Errorf("Realm not set")
	}
	if !mnt.IsSigned() {
		db.DPrintf(db.ERROR, "Dial unsigned mount: %v", mnt)
		return nil, fmt.Errorf("Mount not signed")
	}
	req := &proto.DialRequest{
		Mount: mnt.GetProto(),
	}
	res := &proto.DialResponse{}
	if err := npc.rpcc.RPC("NetProxySrvStubs.Dial", req, res); err != nil {
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

func (npc *NetProxyClnt) proxyListen(addr *sp.Taddr) (*sp.Tmount, net.Listener, error) {
	npc.Lock()
	defer npc.Unlock()

	// Ensure that the connection to the netproxy server has been initialized
	if npc.rpcc == nil {
		if err := npc.init(); err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error dial netproxysrv %v", err)
			return nil, nil, err
		}
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyListen request addr", npc.rpcch.conn)
	req := &proto.ListenRequest{
		Addr: addr,
	}
	res := &proto.ListenResponse{}
	if err := npc.rpcc.RPC("NetProxySrvStubs.Listen", req, res); err != nil {
		return nil, nil, err
	}
	mnt := sp.NewMountFromProto(res.Mount)
	db.DPrintf(db.NETPROXYCLNT, "proxyListen response mnt %v err %v", mnt, res.Err)
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error Listen: %v", err)
		return nil, nil, err
	}
	l, err := npc.rpcch.GetReturnedListener()
	return mnt, l, err
}
