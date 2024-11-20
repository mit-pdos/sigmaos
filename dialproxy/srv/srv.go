package srv

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	dialproxytrans "sigmaos/dialproxy/transport"
	"sigmaos/frame"
	"sigmaos/fs"
	"sigmaos/dialproxy"
	netproto "sigmaos/dialproxy/proto"
	"sigmaos/proc"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/rpcsrv"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type DialProxySrv struct {
	innerContainerIP sp.Tip
	sc               *sigmaclnt.SigmaClnt
	pe               *proc.ProcEnv
}

type DialProxySrvStubs struct {
	closed           bool
	lidctr           dialproxy.Tlidctr
	lm               *dialproxy.ListenerMap
	innerContainerIP sp.Tip
	directDialFn     dialproxy.DialFn
	directListenFn   dialproxy.ListenFn
	p                *sp.Tprincipal
	sc               *sigmaclnt.SigmaClnt
}

type DialProxySrvConn struct {
	npss    *DialProxySrvStubs
	baseCtx *ctx.Ctx
	conn    *net.UnixConn
	trans   *dialproxytrans.DialProxyTrans
	rpcs    *rpcsrv.RPCSrv
	dmx     *demux.DemuxSrv
}

func NewDialProxySrv(pe *proc.ProcEnv) (*DialProxySrv, error) {
	// Create the net proxy socket
	socket, err := net.Listen("unix", sp.SIGMA_DIALPROXY_SOCKET)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(sp.SIGMA_DIALPROXY_SOCKET, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}
	db.DPrintf(db.TEST, "runServer: dialproxysrv listening on %v", sp.SIGMA_DIALPROXY_SOCKET)
	nps := &DialProxySrv{
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

func (nps *DialProxySrv) handleNewConn(conn *net.UnixConn) {
	// Get the principal from the newly established connection
	b, err := frame.ReadFrame(conn)
	if err != nil {
		db.DPrintf(db.DIALPROXYSRV_ERR, "Error Read PrincipalID frame: %v", err)
		return
	}
	p := sp.NoPrincipal()
	if err := json.Unmarshal(b, p); err != nil {
		db.DPrintf(db.ERROR, "Error Unmarshal PrincipalID: %v", err)
		return
	}
	db.DPrintf(db.DIALPROXYSRV, "Handle connection [%p] from principal %v", conn, p)

	npsc := &DialProxySrvConn{
		trans:   dialproxytrans.NewDialProxyTrans(conn, demux.NewIoVecMap()),
		conn:    conn,
		baseCtx: ctx.NewPrincipalOnlyCtx(p),
		npss: &DialProxySrvStubs{
			lm:               dialproxy.NewListenerMap(),
			innerContainerIP: nps.innerContainerIP,
			directListenFn:   dialproxy.ListenDirect,
			sc:               nps.sc,
			p:                p,
		},
	}
	npsc.rpcs = rpcsrv.NewRPCSrv(npsc.npss, rpc.NewStatInfo())
	// Start a demux server to handle requests & concurrency
	npsc.dmx = demux.NewDemuxSrv(npsc, npsc.trans)
}

func (npsc *DialProxySrvConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	db.DPrintf(db.DIALPROXYSRV, "ServeRequest: %v", c)
	req := c.(*dialproxytrans.ProxyCall)
	ctx := dialproxy.NewCtx(npsc.baseCtx)
	rep, err := npsc.rpcs.WriteRead(ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.DIALPROXYSRV, "ServeRequest: writeRead err %v", err)
	}
	npsc.trans.AddConn(req.Seqno, ctx.GetConn())
	return dialproxytrans.NewProxyCall(req.Seqno, rep), nil
}

func (nps *DialProxySrvStubs) Dial(c fs.CtxI, req netproto.DialRequest, res *netproto.DialResponse) error {
	start := time.Now()
	defer func(start time.Time) {
		dur := time.Since(start)
		ep := sp.NewEndpointFromProto(req.GetEndpoint())
		db.DPrintf(db.DIALPROXY_LAT, "[%v] Dial e2e latency: %v", ep, dur)
	}(start)
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*dialproxy.Ctx)
	ep := sp.NewEndpointFromProto(req.GetEndpoint())
	db.DPrintf(db.DIALPROXYSRV, "Dial principal %v -> ep %v", ctx.Principal(), ep)
	proxyConn, err := dialproxy.DialDirect(ctx.Principal(), ep)
	// If Dial was unsuccessful, set the reply error appropriately
	if err != nil {
		db.DPrintf(db.DIALPROXYSRV_ERR, "Error dial direct: %v", err)
		if sr, ok := serr.IsErr(err); ok {
			res.Err = sp.NewRerrorSerr(sr)
		} else {
			res.Err = sp.NewRerrorErr(err)
		}
		return nil
	} else {
		res.Err = sp.NewRerror()
	}
	start = time.Now()
	file, err := dialproxytrans.ConnToFile(proxyConn)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	db.DPrintf(db.DIALPROXY_LAT, "[%v] Dial ConnToFile latency: %v", ep, time.Since(start))
	// Link conn FD to context so that it stays in scope and doesn't get GC-ed
	// before it can be sent back to the client
	ctx.SetConn(file)
	start = time.Now()
	// Set socket control message in output blob
	res.Blob.Iov[0] = dialproxytrans.ConstructSocketControlMsg(file)
	db.DPrintf(db.DIALPROXY_LAT, "[%v] Dial ConstructSocketControlMsg latency: %v", ep, time.Since(start))
	return nil
}

func (nps *DialProxySrvStubs) Listen(c fs.CtxI, req netproto.ListenRequest, res *netproto.ListenResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*dialproxy.Ctx)
	addr := req.GetAddr()
	db.DPrintf(db.DIALPROXYSRV, "Listen principal %v -> addr %v", ctx.Principal(), addr)
	l, err := nps.directListenFn(addr)
	// If Listen was unsuccessful, set the reply error appropriately
	if err != nil {
		db.DPrintf(db.DIALPROXYSRV_ERR, "Error listen direct: %v", err)
		if sr, ok := serr.IsErr(err); ok {
			res.Err = sp.NewRerrorSerr(sr)
		} else {
			res.Err = sp.NewRerrorErr(err)
		}
		return nil
	}
	ep, err := dialproxy.NewEndpoint(sp.TTendpoint(req.EndpointType), nps.innerContainerIP, l)
	if err != nil {
		db.DFatalf("Error construct endpoint: %v", err)
		return err
	}
	res.Endpoint = ep.GetProto()
	// Store the listener & assign it an ID
	lid := dialproxy.Tlid(nps.lidctr.Add(1))
	err = nps.lm.Add(lid, l)
	if err != nil {
		db.DPrintf(db.DIALPROXYSRV_ERR, "Error addListener: %v", err)
		if sr, ok := serr.IsErr(err); ok {
			res.Err = sp.NewRerrorSerr(sr)
		} else {
			res.Err = sp.NewRerrorErr(err)
		}
		l.Close()
		return err
	}
	res.ListenerID = uint64(lid)
	db.DPrintf(db.DIALPROXYSRV, "Listen done principal %v -> addr %v lid %v ep %v", ctx.Principal(), addr, lid, ep)
	res.Err = sp.NewRerror()
	return nil
}

func (nps *DialProxySrvStubs) Accept(c fs.CtxI, req netproto.AcceptRequest, res *netproto.AcceptResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*dialproxy.Ctx)
	lid := dialproxy.Tlid(req.ListenerID)
	db.DPrintf(db.DIALPROXYSRV, "Accept principal %v -> lid %v", ctx.Principal(), lid)
	l, ok := nps.lm.Get(lid)
	if !ok {
		db.DPrintf(db.DIALPROXYSRV_ERR, "Error accept unknown listener %v", lid)
		res.Err = sp.NewRerrorErr(fmt.Errorf("Unknown listener: %v", lid))
		return nil
	}
	// Accept the next connection from a principal authorized to establish a
	// connection to this listener
	proxyConn, p, err := dialproxy.AcceptFromAuthorizedPrincipal(l, req.GetInternalListener(), func(cliP *sp.Tprincipal) bool {
		return dialproxy.ConnectionIsAuthorized(false, nps.p, cliP)
	})
	if err != nil {
		res.Err = sp.NewRerrorErr(fmt.Errorf("Error accept: %v", err))
		return nil
	}
	res.Principal = p
	file, err := dialproxytrans.ConnToFile(proxyConn)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	// Link conn FD to context so that it stays in scope and doesn't get GC-ed
	// before it can be sent back to the client
	ctx.SetConn(file)
	// Set socket control message in output blob
	res.Blob.Iov[0] = dialproxytrans.ConstructSocketControlMsg(file)
	res.Err = sp.NewRerror()
	return nil
}

func (nps *DialProxySrvStubs) Close(c fs.CtxI, req netproto.CloseRequest, res *netproto.CloseResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*dialproxy.Ctx)
	lid := dialproxy.Tlid(req.ListenerID)
	db.DPrintf(db.DIALPROXYSRV, "Close principal %v -> lid %v", ctx.Principal(), lid)
	ok := nps.lm.Close(lid)
	if !ok {
		db.DPrintf(db.DIALPROXYSRV_ERR, "Error close unknown listener %v", lid)
		res.Err = sp.NewRerrorErr(fmt.Errorf("Unknown listener: %v", lid))
		return nil
	}
	res.Err = sp.NewRerror()
	return nil
}

// TODO: check if calling proc cannot invalidate `realm`'s endpoint
func (nps *DialProxySrvStubs) InvalidateNamedEndpointCacheEntry(c fs.CtxI, req netproto.InvalidateNamedEndpointRequest, res *netproto.InvalidateNamedEndpointResponse) error {
	db.DPrintf(db.DIALPROXYSRV, "InvalidateNamedEndpointCacheEntry %v", req)
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	realm := sp.Trealm(req.RealmStr)
	if err := nps.sc.InvalidateNamedEndpointCacheEntryRealm(realm); err != nil {
		db.DPrintf(db.DIALPROXYSRV_ERR, "InvalidateNamedEndpointCacheEntry [%v] err %v %T", realm, err, err)
		if sr, ok := serr.IsErr(err); ok {
			res.Err = sp.NewRerrorSerr(sr)
		} else {
			res.Err = sp.NewRerrorErr(err)
		}
	} else {
		res.Err = sp.NewRerror()
	}
	return nil
}

// TODO: check if calling proc cannot look up `realm`'s endpoint
func (nps *DialProxySrvStubs) GetNamedEndpoint(c fs.CtxI, req netproto.NamedEndpointRequest, res *netproto.NamedEndpointResponse) error {
	db.DPrintf(db.DIALPROXYSRV, "GetNamedEndpoint %v", req)
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	realm := sp.Trealm(req.RealmStr)
	if ep, err := nps.sc.GetNamedEndpointRealm(realm); err != nil {
		db.DPrintf(db.DIALPROXYSRV_ERR, "GetNamedEndpointRealm [%v] err %v %T", realm, err, err)
		if sr, ok := serr.IsErr(err); ok {
			res.Err = sp.NewRerrorSerr(sr)
		} else {
			res.Err = sp.NewRerrorErr(err)
		}
	} else {
		res.Endpoint = ep.GetProto()
		res.Err = sp.NewRerror()
	}
	return nil
}

func (nps *DialProxySrvStubs) closeListeners() error {
	db.DPrintf(db.DIALPROXYSRV, "Close listeners for %v", nps.p)
	return nps.lm.CloseListeners()
}

func (npsc *DialProxySrvConn) ReportError(err error) {
	db.DPrintf(db.DIALPROXYSRV_ERR, "ReportError err %v", err)
	if err := npsc.npss.closeListeners(); err != nil {
		db.DPrintf(db.ERROR, "Err closeListeners: %v", err)
	}
	db.DPrintf(db.DIALPROXYSRV, "Close conn principal %v", npsc.baseCtx.Principal())
	npsc.conn.Close()
}

func (nps *DialProxySrv) Shutdown() {
	db.DPrintf(db.DIALPROXYSRV, "Shutdown")
	os.Remove(sp.SIGMA_DIALPROXY_SOCKET)
}

func (nps *DialProxySrv) runServer(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DFatalf("Error dialproxysrv Accept: %v", err)
			return
		}
		// Handle incoming connection
		go nps.handleNewConn(conn.(*net.UnixConn))
	}
}
