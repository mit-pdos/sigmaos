package netproxyclnt

import (
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"

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
	lidctr          netproxy.Tlidctr
	pe              *proc.ProcEnv
	lm              *netproxy.ListenerMap
	seqcntr         *sessp.Tseqcntr
	trans           *netproxytrans.NetProxyTrans
	dmx             *demux.DemuxClnt
	rpcc            *rpcclnt.RPCClnt
	acceptAllRealms bool // If true, and this netproxyclnt belongs to a kernel proc, accept all realms' connections
}

func NewNetProxyClnt(pe *proc.ProcEnv) *NetProxyClnt {
	return &NetProxyClnt{
		pe:              pe,
		seqcntr:         new(sessp.Tseqcntr),
		lm:              netproxy.NewListenerMap(),
		acceptAllRealms: false,
	}
}

func (npc *NetProxyClnt) AllowConnectionsFromAllRealms() {
	npc.acceptAllRealms = true
}

func (npc *NetProxyClnt) String() string {
	return fmt.Sprintf("{npc useProxy %v}", npc.useProxy())
}

func (npc *NetProxyClnt) Dial(ep *sp.Tendpoint) (net.Conn, error) {
	var c net.Conn
	var err error
	start := time.Now()
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "[%v] proxyDial %v", npc.pe.GetPrincipal(), ep)
		c, err = npc.proxyDial(ep)
		db.DPrintf(db.NETPROXYCLNT, "[%v] proxyDial %v done ok:%v", npc.pe.GetPrincipal(), ep, err == nil)
	} else {
		db.DPrintf(db.NETPROXYCLNT, "[%v] directDial %v", npc.pe.GetPrincipal(), ep)
		c, err = netproxy.DialDirect(npc.pe.GetPrincipal(), ep)
		db.DPrintf(db.NETPROXYCLNT, "[%v] directDial %v done ok:%v", npc.pe.GetPrincipal(), ep, err == nil)
	}
	if err == nil {
		db.DPrintf(db.NETPROXY_LAT, "Dial latency: %v", time.Since(start))
	}
	return c, err
}

func (npc *NetProxyClnt) Listen(ept sp.TTendpoint, addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	var ep *sp.Tendpoint
	var l *Listener
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyListen %v", addr)
		ep, l, err = npc.proxyListen(ept, addr)
		db.DPrintf(db.NETPROXYCLNT, "proxyListen %v done ok:%v", addr, err == nil)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error proxyListen %v: %v", addr, err)
			return nil, nil, err
		}
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directListen %v", addr)
		ep, l, err = npc.directListen(ept, addr)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error directListen %v: %v", addr, err)
			return nil, nil, err
		}
	}
	return ep, l, err
}

func (npc *NetProxyClnt) Accept(lid netproxy.Tlid, internalListener bool) (net.Conn, *sp.Tprincipal, error) {
	var c net.Conn
	var p *sp.Tprincipal
	var err error
	if npc.useProxy() {
		db.DPrintf(db.NETPROXYCLNT, "proxyAccept %v", lid)
		c, p, err = npc.proxyAccept(lid, internalListener)
		db.DPrintf(db.NETPROXYCLNT, "[%v] proxyAccept %v done ok:%v", p, lid, err == nil)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error proxyAccept %v: %v", lid, err)
			return nil, nil, err
		}
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directAccept %v", lid)
		c, p, err = npc.directAccept(lid, internalListener)
		db.DPrintf(db.NETPROXYCLNT, "[%v] directAccept %v done ok:%v", p, lid, err == nil)
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
		db.DPrintf(db.NETPROXYCLNT, "directClose %v", lid)
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

	db.DPrintf(db.NETPROXYCLNT, "[%v] Init netproxyclnt %p", npc.pe.GetPrincipal(), npc)
	defer db.DPrintf(db.NETPROXYCLNT, "[%v] Init netproxyclnt %p done", npc.pe.GetPrincipal(), npc)
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
	start := time.Now()
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, err
	}
	db.DPrintf(db.NETPROXY_LAT, "Dial netproxy conn init: %v", time.Since(start))
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyDial request ep %v", npc.trans.Conn(), ep)
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
	start = time.Now()
	if err := npc.rpcc.RPC("NetProxySrvStubs.Dial", req, res); err != nil {
		return nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "proxyDial response %v", res)
	db.DPrintf(db.NETPROXY_LAT, "Dial netproxy RPC: %v", time.Since(start))
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error Dial: %v", err)
		return nil, err
	}
	start = time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.NETPROXY_LAT, "Dial parseReturnedConn: %v", time.Since(start))
	}(start)
	return netproxytrans.ParseReturnedConn(res.Blob.Iov[0])
}

func (npc *NetProxyClnt) proxyListen(ept sp.TTendpoint, addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	// Ensure that the connection to the netproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyListen request addr %v", npc.trans.Conn(), addr)
	req := &proto.ListenRequest{
		Addr:         addr,
		EndpointType: uint32(ept),
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

func (npc *NetProxyClnt) directListen(ept sp.TTendpoint, addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	l, err := netproxy.ListenDirect(addr)
	if err != nil {
		db.DPrintf(db.ERROR, "Error ListenDirect: %v", err)
		return nil, nil, err
	}
	ep, err := netproxy.NewEndpoint(ept, npc.pe.GetInnerContainerIP(), l)
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

func (npc *NetProxyClnt) directAccept(lid netproxy.Tlid, internalListener bool) (net.Conn, *sp.Tprincipal, error) {
	l, ok := npc.lm.Get(lid)
	if !ok {
		return nil, nil, fmt.Errorf("Unkown direct listener: %v", lid)
	}
	// Accept the next connection from a principal authorized to establish a
	// connection to this listener
	return netproxy.AcceptFromAuthorizedPrincipal(l, internalListener, func(cliP *sp.Tprincipal) bool {
		return netproxy.ConnectionIsAuthorized(npc.acceptAllRealms, npc.pe.GetPrincipal(), cliP)
	})
}

func (npc *NetProxyClnt) proxyAccept(lid netproxy.Tlid, internalListener bool) (net.Conn, *sp.Tprincipal, error) {
	// Ensure that the connection to the netproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyAccept request lip %v", npc.trans.Conn(), lid)
	req := &proto.AcceptRequest{
		ListenerID:       uint64(lid),
		InternalListener: internalListener,
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
