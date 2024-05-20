package netproxysrv

import (
	"fmt"
	"net"
	"os"
	"sync"

	"google.golang.org/protobuf/proto"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/fs"
	"sigmaos/netproxy"
	netproto "sigmaos/netproxy/proto"
	"sigmaos/netproxytrans"
	"sigmaos/proc"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/rpcsrv"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type NetProxySrv struct {
	innerContainerIP sp.Tip
	sc               *sigmaclnt.SigmaClnt
	pe               *proc.ProcEnv
}

type NetProxySrvStubs struct {
	sync.Mutex
	closed           bool
	lidctr           netproxy.Tlidctr
	lm               *netproxy.ListenerMap
	trans            *netproxytrans.NetProxyTrans
	conn             *net.UnixConn
	innerContainerIP sp.Tip
	directDialFn     netproxy.DialFn
	directListenFn   netproxy.ListenFn
	rpcs             *rpcsrv.RPCSrv
	dmx              *demux.DemuxSrv
	baseCtx          *ctx.Ctx
	pe               *proc.ProcEnv
	sc               *sigmaclnt.SigmaClnt
}

func NewNetProxySrv(pe *proc.ProcEnv) (*NetProxySrv, error) {
	// Create the net proxy socket
	socket, err := net.Listen("unix", sp.SIGMA_NETPROXY_SOCKET)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(sp.SIGMA_NETPROXY_SOCKET, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}
	db.DPrintf(db.TEST, "runServer: netproxysrv listening on %v", sp.SIGMA_NETPROXY_SOCKET)
	nps := &NetProxySrv{
		innerContainerIP: pe.GetInnerContainerIP(),
		pe:               pe,
	}
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	nps.sc = sc

	go nps.runServer(socket)

	return nps, nil
}

func (nps *NetProxySrv) handleNewConn(conn *net.UnixConn) {
	// Get the principal from the newly established connection
	b, err := frame.ReadFrame(conn)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Read PrincipalID frame: %v", err)
		return
	}
	p := sp.NoPrincipal()
	if err := proto.Unmarshal(b, p); err != nil {
		db.DPrintf(db.ERROR, "Error Unmarshal PrincipalID: %v", err)
		return
	}
	db.DPrintf(db.NETPROXYSRV, "Handle connection [%p] from principal %v", conn, p)

	npss := &NetProxySrvStubs{
		lm:               netproxy.NewListenerMap(),
		trans:            netproxytrans.NewNetProxyTrans(conn, demux.NewIoVecMap()),
		conn:             conn,
		innerContainerIP: nps.innerContainerIP,
		directListenFn:   netproxy.ListenDirect,
		baseCtx:          ctx.NewPrincipalOnlyCtx(p),
		sc:               nps.sc,
		pe:               nps.pe,
	}
	npss.rpcs = rpcsrv.NewRPCSrv(npss, rpc.NewStatInfo())
	// Start a demux server to handle requests & concurrency
	npss.dmx = demux.NewDemuxSrv(npss, npss.trans)
}

func (npss *NetProxySrvStubs) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	db.DPrintf(db.NETPROXYSRV, "ServeRequest: %v", c)
	req := c.(*netproxytrans.ProxyCall)
	ctx := netproxy.NewCtx(npss.baseCtx)
	rep, err := npss.rpcs.WriteRead(ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV, "ServeRequest: writeRead err %v", err)
	}
	npss.trans.AddConn(req.Seqno, ctx.GetConn())
	return netproxytrans.NewProxyCall(req.Seqno, rep), nil
}

func (nps *NetProxySrvStubs) Dial(c fs.CtxI, req netproto.DialRequest, res *netproto.DialResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*netproxy.Ctx)
	ep := sp.NewEndpointFromProto(req.GetEndpoint())
	db.DPrintf(db.NETPROXYSRV, "Dial principal %v -> ep %v", ctx.Principal(), ep)
	proxyConn, err := netproxy.DialDirect(ctx.Principal(), ep)
	// If Dial was unsuccessful, set the reply error appropriately
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error dial direct: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	} else {
		res.Err = sp.NewRerror()
	}
	file, err := netproxytrans.ConnToFile(proxyConn)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	// Link conn FD to context so that it stays in scope and doesn't get GC-ed
	// before it can be sent back to the client
	ctx.SetConn(file)
	// Set socket control message in output blob
	res.Blob.Iov[0] = netproxytrans.ConstructSocketControlMsg(file)
	return nil
}

func (nps *NetProxySrvStubs) Listen(c fs.CtxI, req netproto.ListenRequest, res *netproto.ListenResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*netproxy.Ctx)
	addr := req.GetAddr()
	db.DPrintf(db.NETPROXYSRV, "Listen principal %v -> addr %v", ctx.Principal(), addr)
	l, err := nps.directListenFn(addr)
	// If Listen was unsuccessful, set the reply error appropriately
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error listen direct: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	}
	ep, err := netproxy.NewEndpoint(sp.TTendpoint(req.EndpointType), nps.innerContainerIP, ctx.Principal().GetRealm(), l)
	if err != nil {
		db.DFatalf("Error construct endpoint: %v", err)
		return err
	}
	res.Endpoint = ep.GetProto()
	// Store the listener & assign it an ID
	lid := netproxy.Tlid(nps.lidctr.Add(1))
	err = nps.lm.Add(lid, l)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error addListener: %v", err)
		res.Err = sp.NewRerrorErr(err)
		l.Close()
		return err
	}
	res.ListenerID = uint64(lid)
	db.DPrintf(db.NETPROXYSRV, "Listen done principal %v -> addr %v lid %v ep %v", ctx.Principal(), addr, lid, ep)
	res.Err = sp.NewRerror()
	return nil
}

func (nps *NetProxySrvStubs) Accept(c fs.CtxI, req netproto.AcceptRequest, res *netproto.AcceptResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*netproxy.Ctx)
	lid := netproxy.Tlid(req.ListenerID)
	db.DPrintf(db.NETPROXYSRV, "Accept principal %v -> lid %v", ctx.Principal(), lid)
	l, ok := nps.lm.Get(lid)
	if !ok {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error accept unknown listener %v", lid)
		res.Err = sp.NewRerrorErr(fmt.Errorf("Unknown listener: %v", lid))
		return nil
	}
	// TODO: optionally, accept without preamble
	proxyConn, p, err := netproxy.AcceptDirect(l, req.GetInternalListener())
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error accept direct: %v", err)
		res.Err = sp.NewRerrorErr(fmt.Errorf("Error accept: %v", err))
		return nil
	}
	res.Principal = p
	file, err := netproxytrans.ConnToFile(proxyConn)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	// Link conn FD to context so that it stays in scope and doesn't get GC-ed
	// before it can be sent back to the client
	ctx.SetConn(file)
	// Set socket control message in output blob
	res.Blob.Iov[0] = netproxytrans.ConstructSocketControlMsg(file)
	res.Err = sp.NewRerror()
	return nil
}

func (nps *NetProxySrvStubs) Close(c fs.CtxI, req netproto.CloseRequest, res *netproto.CloseResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*netproxy.Ctx)
	lid := netproxy.Tlid(req.ListenerID)
	db.DPrintf(db.NETPROXYSRV, "Close principal %v -> lid %v", ctx.Principal(), lid)
	ok := nps.lm.Close(lid)
	if !ok {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error close unknown listener %v", lid)
		res.Err = sp.NewRerrorErr(fmt.Errorf("Unknown listener: %v", lid))
		return nil
	}
	res.Err = sp.NewRerror()
	return nil
}

// TODO: check if calling proc cannot invalidate `realm`'s endpoint
func (nps *NetProxySrvStubs) InvalidateNamedEndpointCacheEntry(c fs.CtxI, req netproto.InvalidateNamedEndpointRequest, res *netproto.InvalidateNamedEndpointResponse) error {
	db.DPrintf(db.NETPROXYSRV, "InvalidateNamedEndpointCacheEntry %v", req)
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	realm := sp.Trealm(req.RealmStr)
	if err := nps.sc.InvalidateNamedEndpointCacheEntryRealm(realm); err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "InvalidateNamedEndpointCacheEntry [%v] err %v %T", realm, err, err)
		res.Err = sp.NewRerrorErr(err)
	} else {
		res.Err = sp.NewRerror()
	}
	return nil
}

// TODO: check if calling proc cannot look up `realm`'s endpoint
func (nps *NetProxySrvStubs) GetNamedEndpoint(c fs.CtxI, req netproto.NamedEndpointRequest, res *netproto.NamedEndpointResponse) error {
	db.DPrintf(db.NETPROXYSRV, "GetNamedEndpoint %v", req)
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	realm := sp.Trealm(req.RealmStr)
	if ep, err := nps.sc.GetNamedEndpointRealm(realm); err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "GetNamedEndpointRealm [%v] err %v %T", realm, err, err)
		res.Err = sp.NewRerrorErr(err)
	} else {
		res.Endpoint = ep.GetProto()
		res.Err = sp.NewRerror()
	}
	return nil
}

func (nps *NetProxySrvStubs) closeListeners() error {
	db.DPrintf(db.NETPROXYSRV, "Close listeners for %v", nps.pe.GetPID())
	return nps.lm.CloseListeners()
}

func (npss *NetProxySrvStubs) ReportError(err error) {
	db.DPrintf(db.NETPROXYSRV_ERR, "ReportError err %v", err)
	if err := npss.closeListeners(); err != nil {
		db.DPrintf(db.ERROR, "Err closeListeners: %v", err)
	}
	db.DPrintf(db.NETPROXYSRV, "Close conn principal %v", npss.baseCtx.Principal())
	npss.conn.Close()
}

func (nps *NetProxySrv) Shutdown() {
	db.DPrintf(db.NETPROXYSRV, "Shutdown")
	os.Remove(sp.SIGMA_NETPROXY_SOCKET)
}

func (nps *NetProxySrv) runServer(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DFatalf("Error netproxysrv Accept: %v", err)
			return
		}
		// Handle incoming connection
		go nps.handleNewConn(conn.(*net.UnixConn))
	}
}
