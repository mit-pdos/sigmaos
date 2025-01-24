package clnt

import (
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/dialproxy"
	"sigmaos/dialproxy/proto"
	dialproxytrans "sigmaos/dialproxy/transport"
	"sigmaos/proc"
	"sigmaos/rpc"
	rpcclnt "sigmaos/rpc/clnt"
	"sigmaos/rpc/clnt/opts"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type DialProxyClnt struct {
	sync.Mutex
	lidctr          dialproxy.Tlidctr
	pe              *proc.ProcEnv
	lm              *dialproxy.ListenerMap
	seqcntr         *sessp.Tseqcntr
	trans           *dialproxytrans.DialProxyTrans
	dmx             *demux.DemuxClnt
	rpcc            *rpcclnt.RPCClnt
	acceptAllRealms bool // If true, and this dialproxyclnt belongs to a kernel proc, accept all realms' connections
}

func NewDialProxyClnt(pe *proc.ProcEnv) *DialProxyClnt {
	return &DialProxyClnt{
		pe:              pe,
		seqcntr:         new(sessp.Tseqcntr),
		lm:              dialproxy.NewListenerMap(),
		acceptAllRealms: false,
	}
}

// Get the connection underlying the transport to dialproxyd
func (npc *DialProxyClnt) GetDialProxyConn() *net.UnixConn {
	return npc.trans.Conn()
}

func (npc *DialProxyClnt) AllowConnectionsFromAllRealms() {
	npc.acceptAllRealms = true
}

func (npc *DialProxyClnt) String() string {
	return fmt.Sprintf("{npc useProxy %v}", npc.useProxy())
}

func (npc *DialProxyClnt) Dial(ep *sp.Tendpoint) (net.Conn, error) {
	var c net.Conn
	var err error
	start := time.Now()
	if npc.useProxy() {
		db.DPrintf(db.DIALPROXYCLNT, "[%v] proxyDial %v", npc.pe.GetPrincipal(), ep)
		c, err = npc.proxyDial(ep)
		db.DPrintf(db.DIALPROXYCLNT, "[%v] proxyDial %v done ok:%v", npc.pe.GetPrincipal(), ep, err == nil)
	} else {
		db.DPrintf(db.DIALPROXYCLNT, "[%v] directDial %v", npc.pe.GetPrincipal(), ep)
		c, err = dialproxy.DialDirect(npc.pe.GetPrincipal(), ep)
		db.DPrintf(db.DIALPROXYCLNT, "[%v] directDial %v done ok:%v", npc.pe.GetPrincipal(), ep, err == nil)
	}
	if err == nil {
		db.DPrintf(db.DIALPROXY_LAT, "Dial latency: %v", time.Since(start))
	}
	return c, err
}

func (npc *DialProxyClnt) Listen(ept sp.TTendpoint, addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	var ep *sp.Tendpoint
	var l *Listener
	var err error
	if npc.useProxy() {
		db.DPrintf(db.DIALPROXYCLNT, "proxyListen %v", addr)
		ep, l, err = npc.proxyListen(ept, addr)
		db.DPrintf(db.DIALPROXYCLNT, "proxyListen %v done ok:%v", addr, err == nil)
		if err != nil {
			db.DPrintf(db.DIALPROXYCLNT_ERR, "Error proxyListen %v: %v", addr, err)
			return nil, nil, err
		}
	} else {
		db.DPrintf(db.DIALPROXYCLNT, "directListen %v", addr)
		ep, l, err = npc.directListen(ept, addr)
		if err != nil {
			db.DPrintf(db.DIALPROXYCLNT_ERR, "Error directListen %v: %v", addr, err)
			return nil, nil, err
		}
	}
	return ep, l, err
}

func (npc *DialProxyClnt) Accept(lid dialproxy.Tlid, internalListener bool) (net.Conn, *sp.Tprincipal, error) {
	var c net.Conn
	var p *sp.Tprincipal
	var err error
	if npc.useProxy() {
		db.DPrintf(db.DIALPROXYCLNT, "proxyAccept %v", lid)
		c, p, err = npc.proxyAccept(lid, internalListener)
		db.DPrintf(db.DIALPROXYCLNT, "[%v] proxyAccept %v done ok:%v", p, lid, err == nil)
		if err != nil {
			db.DPrintf(db.DIALPROXYCLNT_ERR, "Error proxyAccept %v: %v", lid, err)
			return nil, nil, err
		}
	} else {
		db.DPrintf(db.DIALPROXYCLNT, "directAccept %v", lid)
		c, p, err = npc.directAccept(lid, internalListener)
		db.DPrintf(db.DIALPROXYCLNT, "[%v] directAccept %v done ok:%v", p, lid, err == nil)
		if err != nil {
			db.DPrintf(db.DIALPROXYCLNT_ERR, "Error directAccept %v: %v", lid, err)
			return nil, nil, err
		}
	}
	return c, p, err
}

func (npc *DialProxyClnt) Close(lid dialproxy.Tlid) error {
	var err error
	if npc.useProxy() {
		db.DPrintf(db.DIALPROXYCLNT, "proxyClose %v", lid)
		err = npc.proxyClose(lid)
		db.DPrintf(db.DIALPROXYCLNT, "proxyClose %v done ok:%v", lid, err == nil)
		if err != nil {
			db.DPrintf(db.DIALPROXYCLNT_ERR, "Error proxyClose %v: %v", lid, err)
			return err
		}
	} else {
		db.DPrintf(db.DIALPROXYCLNT, "directClose %v", lid)
		err = npc.directClose(lid)
		if err != nil {
			db.DPrintf(db.DIALPROXYCLNT_ERR, "Error directClose %v: %v", lid, err)
			return err
		}
	}
	return err
}

// If true, use the net proxy server for dialing & listening.
func (npc *DialProxyClnt) useProxy() bool {
	npc.Lock()
	defer npc.Unlock()

	return npc.pe.GetUseDialProxy()
}

// Lazily init connection to the dialproxy srv
func (npc *DialProxyClnt) init() error {
	npc.Lock()
	defer npc.Unlock()

	// If rpc clnt has already been initialized, bail out
	if npc.rpcc != nil {
		return nil
	}

	db.DPrintf(db.DIALPROXYCLNT, "[%v] Init dialproxyclnt %p", npc.pe.GetPrincipal(), npc)
	defer db.DPrintf(db.DIALPROXYCLNT, "[%v] Init dialproxyclnt %p done", npc.pe.GetPrincipal(), npc)
	iovm := demux.NewIoVecMap()
	conn, err := dialproxytrans.GetDialProxydConn(npc.pe)
	if err != nil {
		return err
	}
	// Connect to the dialproxy server
	trans := dialproxytrans.NewDialProxyTrans(conn, iovm)
	npc.trans = trans
	npc.dmx = demux.NewDemuxClnt(trans, iovm)
	rpcc, err := rpcclnt.NewRPCClnt("no-path", opts.WithRPCChannel(npc))
	if err != nil {
		return err
	}
	npc.rpcc = rpcc
	return nil
}

func (npc *DialProxyClnt) proxyDial(ep *sp.Tendpoint) (net.Conn, error) {
	// Ensure that the connection to the dialproxy server has been initialized
	start := time.Now()
	if err := npc.init(); err != nil {
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error init dialproxyclnt %v", err)
		return nil, err
	}
	db.DPrintf(db.DIALPROXY_LAT, "Dial dialproxy conn init: %v", time.Since(start))
	db.DPrintf(db.DIALPROXYCLNT, "[%p] proxyDial request ep %v", npc.trans.Conn(), ep)
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
	if err := npc.rpcc.RPC("DialProxySrvStubs.Dial", req, res); err != nil {
		return nil, err
	}
	db.DPrintf(db.DIALPROXYCLNT, "proxyDial response %v", res)
	db.DPrintf(db.DIALPROXY_LAT, "Dial dialproxy RPC: %v", time.Since(start))
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error Dial: %v", err)
		return nil, err
	}
	start = time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.DIALPROXY_LAT, "Dial parseReturnedConn: %v", time.Since(start))
	}(start)
	return dialproxytrans.ParseReturnedConn(res.Blob.Iov[0])
}

func (npc *DialProxyClnt) proxyListen(ept sp.TTendpoint, addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	// Ensure that the connection to the dialproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error init dialproxyclnt %v", err)
		return nil, nil, err
	}
	db.DPrintf(db.DIALPROXYCLNT, "[%p] proxyListen request addr %v", npc.trans.Conn(), addr)
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
	if err := npc.rpcc.RPC("DialProxySrvStubs.Listen", req, res); err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.DIALPROXYCLNT, "proxyListen response %v", res)
	// If an error occurred during listen, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error Listen: %v", err)
		return nil, nil, err
	}
	if res.ListenerID == 0 {
		db.DPrintf(db.ERROR, "No listener ID returned")
		return nil, nil, fmt.Errorf("No listener ID")
	}
	ep := sp.NewEndpointFromProto(res.Endpoint)
	return ep, NewListener(npc, dialproxy.Tlid(res.ListenerID), ep), nil
}

func (npc *DialProxyClnt) directListen(ept sp.TTendpoint, addr *sp.Taddr) (*sp.Tendpoint, *Listener, error) {
	l, err := dialproxy.ListenDirect(addr)
	if err != nil {
		db.DPrintf(db.ERROR, "Error ListenDirect: %v", err)
		return nil, nil, err
	}
	ep, err := dialproxy.NewEndpoint(ept, npc.pe.GetInnerContainerIP(), l)
	if err != nil {
		db.DPrintf(db.ERROR, "Error construct endpoint: %v", err)
		return nil, nil, err
	}
	// Add to the listener map
	lid := dialproxy.Tlid(npc.lidctr.Add(1))
	db.DPrintf(db.DIALPROXYCLNT, "Listen lid %v ep %v", lid, ep)
	npc.lm.Add(lid, l)
	return ep, NewListener(npc, lid, ep), err
}

func (npc *DialProxyClnt) directAccept(lid dialproxy.Tlid, internalListener bool) (net.Conn, *sp.Tprincipal, error) {
	l, ok := npc.lm.Get(lid)
	if !ok {
		return nil, nil, fmt.Errorf("Unkown direct listener: %v", lid)
	}
	// Accept the next connection from a principal authorized to establish a
	// connection to this listener
	return dialproxy.AcceptFromAuthorizedPrincipal(l, internalListener, func(cliP *sp.Tprincipal) bool {
		return dialproxy.ConnectionIsAuthorized(npc.acceptAllRealms, npc.pe.GetPrincipal(), cliP)
	})
}

func (npc *DialProxyClnt) proxyAccept(lid dialproxy.Tlid, internalListener bool) (net.Conn, *sp.Tprincipal, error) {
	// Ensure that the connection to the dialproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error init dialproxyclnt %v", err)
		return nil, nil, err
	}
	db.DPrintf(db.DIALPROXYCLNT, "[%p] proxyAccept request lip %v", npc.trans.Conn(), lid)
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
	if err := npc.rpcc.RPC("DialProxySrvStubs.Accept", req, res); err != nil {
		return nil, nil, err
	}
	db.DPrintf(db.DIALPROXYCLNT, "proxyAccept response %v", res)
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error Accept: %v", err)
		return nil, nil, err
	}
	conn, err := dialproxytrans.ParseReturnedConn(res.Blob.Iov[0])
	if err != nil {
		return nil, nil, err
	}
	return conn, res.GetPrincipal(), err
}

func (npc *DialProxyClnt) directClose(lid dialproxy.Tlid) error {
	if ok := npc.lm.Close(lid); !ok {
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error close unknown listener: %v", lid)
		return fmt.Errorf("Close unknown listener: %v", lid)
	}
	return nil
}

func (npc *DialProxyClnt) proxyClose(lid dialproxy.Tlid) error {
	// Ensure that the connection to the dialproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error init dialproxyclnt %v", err)
		return err
	}
	db.DPrintf(db.DIALPROXYCLNT, "[%p] proxyClose request lip %v", npc.trans.Conn(), lid)
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
	if err := npc.rpcc.RPC("DialProxySrvStubs.Close", req, res); err != nil {
		return err
	}
	db.DPrintf(db.DIALPROXYCLNT, "proxyClose response %v", res)
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error Close: %v", err)
		return err
	}
	return nil
}

func (npc *DialProxyClnt) newListener(lid dialproxy.Tlid) net.Listener {
	db.DFatalf("Unimplemented")
	return nil
}

func (npc *DialProxyClnt) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
	c := dialproxytrans.NewProxyCall(sessp.NextSeqno(npc.seqcntr), iniov)
	rep, err := npc.dmx.SendReceive(c, outiov)
	if err != nil {
		return err
	} else {
		c := rep.(*dialproxytrans.ProxyCall)
		if len(outiov) != len(c.Iov) {
			return fmt.Errorf("dialproxyclnt outiov len wrong: %v != %v", len(outiov), len(c.Iov))
		}
		return nil
	}
}

func (npc *DialProxyClnt) GetNamedEndpoint(r sp.Trealm) (*sp.Tendpoint, error) {
	if !npc.useProxy() {
		db.DFatalf("GetNamedEndpoint: internal %v\n", npc)
	}
	ep, err := npc.getNamedEndpoint(r)
	db.DPrintf(db.DIALPROXYCLNT, "GetNamedEndpoint %v ep %v err %v", r, ep, err)
	if err != nil {
		return nil, err
	}
	return ep, nil
}

func (npc *DialProxyClnt) getNamedEndpoint(r sp.Trealm) (*sp.Tendpoint, error) {
	if err := npc.init(); err != nil {
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error init dialproxyclnt %v", err)
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
	if err := npc.rpcc.RPC("DialProxySrvStubs.GetNamedEndpoint", req, res); err != nil {
		return nil, err
	}
	if res.Err.ErrCode != 0 {
		return nil, sp.NewErr(res.Err)
	} else {
		ep := sp.NewEndpointFromProto(res.Endpoint)
		return ep, nil
	}
}

func (npc *DialProxyClnt) invalidateNamedEndpointCacheEntry(r sp.Trealm) error {
	if err := npc.init(); err != nil {
		db.DPrintf(db.DIALPROXYCLNT_ERR, "Error init dialproxyclnt %v", err)
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
	db.DPrintf(db.DIALPROXYCLNT, "rpcing dialproxyclnt %v", r)
	if err := npc.rpcc.RPC("DialProxySrvStubs.InvalidateNamedEndpointCacheEntry", req, res); err != nil {
		db.DPrintf(db.DIALPROXYCLNT, "rpced dialproxyclnt %v err %v", r, err)
		return err
	}
	db.DPrintf(db.DIALPROXYCLNT, "rpc dialproxyclnt %v no err", r)
	if res.Err.ErrCode != 0 {
		return sp.NewErr(res.Err)
	} else {
		return nil
	}
}

func (npc *DialProxyClnt) InvalidateNamedEndpointCacheEntry(r sp.Trealm) error {
	if !npc.useProxy() {
		db.DFatalf("InvalidateNamedEndpointCacheEntry: internal %v\n", npc)
	}
	db.DPrintf(db.DIALPROXYCLNT, "InvalidateNamedEndpointCacheEntry %v", r)
	err := npc.invalidateNamedEndpointCacheEntry(r)
	db.DPrintf(db.DIALPROXYCLNT, "InvalidatedNamedEndpointCacheEntry %v", r)
	if err != nil {
		db.DPrintf(db.DIALPROXYCLNT_ERR, "InvalidateNamedEndpointCacheEntry %v err %v", r, err)
		return err
	}
	return nil
}

func (npc *DialProxyClnt) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	db.DPrintf(db.ERROR, "StatsSrv unimplemented")
	return nil, fmt.Errorf("Unimplemented")
}
