package netproxyclnt

import (
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"runtime/debug"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netproxy"
	"sigmaos/netproxy/proto"
	"sigmaos/netproxytrans"
	"sigmaos/proc"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/rpcclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type NetProxyClnt struct {
	sync.Mutex
	lidctr           netproxy.Tlidctr
	pe               *proc.ProcEnv
	canSignEndpoints bool
	verifyEndpoints  bool
	auth             auth.AuthMgr
	lm               *netproxy.ListenerMap
	seqcntr          *sessp.Tseqcntr
	trans            *netproxytrans.NetProxyTrans
	dmx              *demux.DemuxClnt
	rpcc             *rpcclnt.RPCClnt
}

func NewNetProxyClnt(pe *proc.ProcEnv, amgr auth.AuthMgr) *NetProxyClnt {
	return &NetProxyClnt{
		pe:               pe,
		canSignEndpoints: amgr != nil,
		verifyEndpoints:  pe.GetVerifyEndpoints(),
		auth:             amgr,
		seqcntr:          new(sessp.Tseqcntr),
		lm:               netproxy.NewListenerMap(),
	}
}

func (npc *NetProxyClnt) SetAuthMgr(amgr auth.AuthMgr) {
	npc.Lock()
	defer npc.Unlock()

	npc.auth = amgr
	npc.canSignEndpoints = true
}

func (npc *NetProxyClnt) GetAuthMgr() auth.AuthMgr {
	npc.Lock()
	defer npc.Unlock()

	return npc.auth
}

func (npc *NetProxyClnt) Dial(ep *sp.Tendpoint) (net.Conn, error) {
	var c net.Conn
	var err error
	start := time.Now()
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyDial %v", ep)
		c, err = npc.proxyDial(ep)
		db.DPrintf(db.NETPROXYCLNT, "proxyDial %v done ok:%v", ep, err == nil)
	} else {
		// TODO: send PE (or just realm) here
		db.DPrintf(db.NETPROXYCLNT, "directDial %v", ep)
		c, err = netproxy.DialDirect(ep)
		db.DPrintf(db.NETPROXYCLNT, "directDial %v done ok:%v", ep, err == nil)
	}
	if err == nil {
		db.DPrintf(db.NETSIGMA_PERF, "Dial latency: %v", time.Since(start))
	}
	return c, err
}

// TODO: return *netproxy.Listener, not net.Listener
func (npc *NetProxyClnt) Listen(addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	var ep *sp.Tendpoint
	var l *Listener
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyListen %v", addr)
		ep, l, err = npc.proxyListen(addr)
		db.DPrintf(db.NETPROXYCLNT, "proxyListen %v done ok:%v", addr, err == nil)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error proxyListen %v: %v", addr, err)
			return nil, nil, err
		}
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directListen %v", addr)
		if npc.verifyEndpoints && !npc.canSignEndpoints {
			err := fmt.Errorf("Try to listen on netproxyclnt without AuthMgr")
			db.DPrintf(db.ERROR, "Err listen: %v", err)
			return nil, nil, err
		}
		ep, l, err = npc.directListen(addr)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error directListen %v: %v", addr, err)
			return nil, nil, err
		}
	}
	return ep, l, err
}

func (npc *NetProxyClnt) Accept(lid netproxy.Tlid) (net.Conn, *sp.Tprincipal, error) {
	var c net.Conn
	var p *sp.Tprincipal
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyAccept %v", lid)
		c, p, err = npc.proxyAccept(lid)
		db.DPrintf(db.NETPROXYCLNT, "proxyAccept %v done ok:%v", lid, err == nil)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error proxyAccept %v: %v", lid, err)
			return nil, nil, err
		}
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directAccept %v", lid)
		c, p, err = npc.directAccept(lid)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error directAccept %v: %v", lid, err)
			return nil, nil, err
		}
	}
	return c, p, err
}

func (npc *NetProxyClnt) Close(lid netproxy.Tlid) error {
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyClose %v", lid)
		err = npc.proxyClose(lid)
		db.DPrintf(db.NETPROXYCLNT, "proxyClose %v done ok:%v", lid, err == nil)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error proxyClose %v: %v", lid, err)
			return err
		}
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directClose %v stack:\n%v", lid, string(debug.Stack()))
		err = npc.directClose(lid)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error directClose %v: %v", lid, err)
			return err
		}
	}
	return err
}

// If true, use the net proxy server for dialing & listening.
func (npc *NetProxyClnt) useProxy() bool {
	npc.Lock()
	defer npc.Unlock()

	return npc.pe.GetUseNetProxy()
}

// Lazily init connection to the netproxy srv
func (npc *NetProxyClnt) init() error {
	npc.Lock()
	defer npc.Unlock()

	// If rpc clnt has already been initialized, bail out
	if npc.rpcc != nil {
		return nil
	}

	db.DPrintf(db.NETPROXYCLNT, "Init netproxyclnt %p", npc)
	iovm := demux.NewIoVecMap()
	conn, err := netproxytrans.GetNetproxydConn(npc.pe)
	if err != nil {
		return err
	}
	// Connect to the netproxy server
	trans := netproxytrans.NewNetProxyTrans(conn, iovm)
	npc.trans = trans
	npc.dmx = demux.NewDemuxClnt(trans, iovm)
	npc.rpcc = rpcclnt.NewRPCClnt(npc)
	return nil
}

func (npc *NetProxyClnt) proxyDial(ep *sp.Tendpoint) (net.Conn, error) {
	// Ensure that the connection to the netproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyDial request ep %v", npc.trans.Conn(), ep)
	// Endpoints should always have realms specified
	if ep.GetRealm() == sp.NOT_SET {
		db.DPrintf(db.ERROR, "Dial endpoint without realm set: %v", ep)
		return nil, fmt.Errorf("Realm not set")
	}
	if !ep.IsSigned() && npc.verifyEndpoints {
		db.DPrintf(db.ERROR, "Dial unsigned endpoint: %v", ep)
		return nil, fmt.Errorf("Endpoint not signed")
	}
	req := &proto.DialRequest{
		Endpoint: ep.GetProto(),
		// Requests must have blob too, so that unix sendmsg works
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	// Set up the blob to receive the socket control message
	res := &proto.DialResponse{
		Blob: &rpcproto.Blob{
			Iov: [][]byte{make([]byte, unix.CmsgSpace(4))},
		},
	}
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
	return netproxytrans.ParseReturnedConn(res.Blob.Iov[0])
}

func (npc *NetProxyClnt) proxyListen(addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	// Ensure that the connection to the netproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyListen request addr %v", npc.trans.Conn(), addr)
	req := &proto.ListenRequest{
		Addr: addr,
		// Requests must have blob too, so that unix sendmsg works
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	// Set up the blob to receive the socket control message
	res := &proto.ListenResponse{
		Blob: &rpcproto.Blob{
			Iov: [][]byte{make([]byte, unix.CmsgSpace(4))},
		},
	}
	if err := npc.rpcc.RPC("NetProxySrvStubs.Listen", req, res); err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "proxyListen response %v", res)
	// If an error occurred during listen, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error Listen: %v", err)
		return nil, nil, err
	}
	if res.ListenerID == 0 {
		db.DPrintf(db.ERROR, "No listener ID returned")
		return nil, nil, fmt.Errorf("No listener ID")
	}
	ep := sp.NewEndpointFromProto(res.Endpoint)
	return ep, NewListener(npc, netproxy.Tlid(res.ListenerID), ep), nil
}

func (npc *NetProxyClnt) directListen(addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	l, err := netproxy.ListenDirect(addr)
	if err != nil {
		db.DPrintf(db.ERROR, "Error ListenDirect: %v", err)
		return nil, nil, err
	}
	ep, err := netproxy.NewEndpoint(npc.verifyEndpoints, npc.auth, npc.pe.GetInnerContainerIP(), npc.pe.GetRealm(), l)
	if err != nil {
		db.DPrintf(db.ERROR, "Error construct endpoint: %v", err)
		return nil, nil, err
	}
	// Add to the listener map
	lid := netproxy.Tlid(npc.lidctr.Add(1))
	db.DPrintf(db.NETPROXYCLNT, "Listen lid %v ep %v", lid, ep)
	npc.lm.Add(lid, l)
	return ep, NewListener(npc, lid, ep), err
}

func (npc *NetProxyClnt) directAccept(lid netproxy.Tlid) (net.Conn, *sp.Tprincipal, error) {
	l, ok := npc.lm.Get(lid)
	if !ok {
		return nil, nil, fmt.Errorf("Unkown direct listener: %v", lid)
	}
	c, err := l.Accept()
	if err != nil {
		return nil, nil, err
	}
	// TODO: get principal from conn
	return c, nil, err
}

func (npc *NetProxyClnt) proxyAccept(lid netproxy.Tlid) (net.Conn, *sp.Tprincipal, error) {
	// Ensure that the connection to the netproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyAccept request lip %v", npc.trans.Conn(), lid)
	req := &proto.AcceptRequest{
		ListenerID: uint64(lid),
		// Requests must have blob too, so that unix sendmsg works
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	// Set up the blob to receive the socket control message
	res := &proto.AcceptResponse{
		Blob: &rpcproto.Blob{
			Iov: [][]byte{make([]byte, unix.CmsgSpace(4))},
		},
	}
	if err := npc.rpcc.RPC("NetProxySrvStubs.Accept", req, res); err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "proxyAccept response %v", res)
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error Accept: %v", err)
		return nil, nil, err
	}
	conn, err := netproxytrans.ParseReturnedConn(res.Blob.Iov[0])
	if err != nil {
		return nil, nil, err
	}
	return conn, res.GetPrincipal(), err
}

func (npc *NetProxyClnt) directClose(lid netproxy.Tlid) error {
	if ok := npc.lm.Close(lid); !ok {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error close unknown listener: %v", lid)
		return fmt.Errorf("Close unknown listener: %v", lid)
	}
	return nil
}

func (npc *NetProxyClnt) proxyClose(lid netproxy.Tlid) error {
	// Ensure that the connection to the netproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return err
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyClose request lip %v", npc.trans.Conn(), lid)
	req := &proto.CloseRequest{
		ListenerID: uint64(lid),
		// Requests must have blob too, so that unix sendmsg works
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	// Set up the blob to receive the socket control message
	res := &proto.CloseResponse{
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	if err := npc.rpcc.RPC("NetProxySrvStubs.Close", req, res); err != nil {
		return err
	}
	db.DPrintf(db.NETPROXYCLNT, "proxyClose response %v", res)
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error Close: %v", err)
		return err
	}
	return nil
}

func (npc *NetProxyClnt) newListener(lid netproxy.Tlid) net.Listener {
	db.DFatalf("Unimplemented")
	return nil
}

func (npc *NetProxyClnt) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
	c := netproxytrans.NewProxyCall(sessp.NextSeqno(npc.seqcntr), iniov)
	rep, err := npc.dmx.SendReceive(c, outiov)
	if err != nil {
		return err
	} else {
		c := rep.(*netproxytrans.ProxyCall)
		if len(outiov) != len(c.Iov) {
			return fmt.Errorf("netproxyclnt outiov len wrong: %v != %v", len(outiov), len(c.Iov))
		}
		return nil
	}
}

func (npc *NetProxyClnt) GetNamedEndpoint(r sp.Trealm) (*sp.Tendpoint, error) {
	if !npc.useProxy() {
		db.DFatalf("GetNamedEndpoint: internal %v\n", npc)
	}
	db.DPrintf(db.NETPROXYCLNT, "GetNamedEndpoint %v", r)
	ep, err := npc.getNamedEndpoint(r)
	if err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "GetNamedEndpoint %v err %v", r, err)
		return nil, err
	}
	return ep, nil
}

func (npc *NetProxyClnt) getNamedEndpoint(r sp.Trealm) (*sp.Tendpoint, error) {
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, err
	}
	req := &proto.NamedEndpointRequest{
		RealmStr: r.String(),
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	res := &proto.NamedEndpointResponse{
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	if err := npc.rpcc.RPC("NetProxySrvStubs.GetNamedEndpoint", req, res); err != nil {
		return nil, err
	}
	if res.Err.ErrCode != 0 {
		return nil, sp.NewErr(res.Err)
	} else {
		ep := sp.NewEndpointFromProto(res.Endpoint)
		return ep, nil
	}
}

func (npc *NetProxyClnt) invalidateNamedEndpointCacheEntry(r sp.Trealm) error {
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return err
	}
	req := &proto.InvalidateNamedEndpointRequest{
		RealmStr: r.String(),
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	res := &proto.InvalidateNamedEndpointResponse{
		Blob: &rpcproto.Blob{
			Iov: [][]byte{nil},
		},
	}
	if err := npc.rpcc.RPC("NetProxySrvStubs.InvalidateNamedEndpointCacheEntry", req, res); err != nil {
		return err
	}
	if res.Err.ErrCode != 0 {
		return sp.NewErr(res.Err)
	} else {
		return nil
	}
}

func (npc *NetProxyClnt) InvalidateNamedEndpointCacheEntry(r sp.Trealm) error {
	if !npc.useProxy() {
		db.DFatalf("InvalidateNamedEndpointCacheEntry: internal %v\n", npc)
	}
	db.DPrintf(db.NETPROXYCLNT, "InvalidateNamedEndpointCacheEntry %v", r)
	err := npc.invalidateNamedEndpointCacheEntry(r)
	if err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "InvalidateNamedEndpointCacheEntry %v err %v", r, err)
		return err
	}
	return nil
}

func (npc *NetProxyClnt) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	db.DPrintf(db.ERROR, "StatsSrv unimplemented")
	return nil, fmt.Errorf("Unimplemented")
}
