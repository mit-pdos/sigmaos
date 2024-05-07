package netproxy

import (
	"fmt"
	"net"
	"os"
	"sync"

	"google.golang.org/protobuf/proto"

	"sigmaos/auth"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/fs"
	netproto "sigmaos/netproxy/proto"
	"sigmaos/netsigma"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/rpcsrv"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type NetProxySrv struct {
	auth             auth.AuthMgr
	innerContainerIP sp.Tip
}

type NetProxySrvStubs struct {
	sync.Mutex
	lidctr           Tlidctr
	listeners        map[Tlid]net.Listener
	trans            *NetProxyTrans
	conn             *net.UnixConn
	innerContainerIP sp.Tip
	auth             auth.AuthMgr
	directDialFn     DialFn
	directListenFn   ListenFn
	rpcs             *rpcsrv.RPCSrv
	dmx              *demux.DemuxSrv
	baseCtx          *ctx.Ctx
}

func NewNetProxySrv(ip sp.Tip, amgr auth.AuthMgr) (*NetProxySrv, error) {
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
		auth:             amgr,
		innerContainerIP: ip,
	}

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
		listeners:        make(map[Tlid]net.Listener),
		trans:            NewNetProxyTrans(conn, demux.NewIoVecMap()),
		auth:             nps.auth,
		conn:             conn,
		innerContainerIP: nps.innerContainerIP,
		directDialFn:     DialDirect,
		directListenFn:   ListenDirect,
		baseCtx:          ctx.NewPrincipalOnlyCtx(p),
	}
	npss.rpcs = rpcsrv.NewRPCSrv(npss, rpc.NewStatInfo())
	// Start a demux server to handle requests & concurrency
	npss.dmx = demux.NewDemuxSrv(npss, npss.trans)
}

func (npss *NetProxySrvStubs) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	db.DPrintf(db.NETPROXYSRV, "ServeRequest: %v", c)
	req := c.(*ProxyCall)
	ctx := NewCtx(npss.baseCtx)
	rep, err := npss.rpcs.WriteRead(ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV, "ServeRequest: writeRead err %v", err)
	}
	npss.trans.AddConn(req.Seqno, ctx.GetConn())
	return NewProxyCall(req.Seqno, rep), nil
}

func (nps *NetProxySrvStubs) Dial(c fs.CtxI, req netproto.DialRequest, res *netproto.DialResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*Ctx)
	ep := sp.NewEndpointFromProto(req.GetEndpoint())
	db.DPrintf(db.NETPROXYSRV, "Dial principal %v -> ep %v", ctx.Principal(), ep)
	// Verify the principal is authorized to establish the connection
	// XXX skip verif
	//	if _, err := nps.auth.EndpointIsAuthorized(ctx.Principal(), ep); err != nil {
	//		db.DPrintf(db.NETPROXYSRV_ERR, "Error Dial unauthorized endpoint: %v", err)
	//		res.Err = sp.NewRerrorErr(err)
	//		return nil
	//	}
	proxyConn, err := nps.directDialFn(ep)
	// If Dial was unsuccessful, set the reply error appropriately
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error dial direct: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	} else {
		res.Err = sp.NewRerror()
	}
	file, err := connToFile(proxyConn)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	// Link conn FD to context so that it stays in scope and doesn't get GC-ed
	// before it can be sent back to the client
	ctx.SetConn(file)
	// Set socket control message in output blob
	res.Blob.Iov[0] = constructSocketControlMsg(file)
	return nil
}

func (nps *NetProxySrvStubs) Listen(c fs.CtxI, req netproto.ListenRequest, res *netproto.ListenResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*Ctx)
	addr := req.GetAddr()
	db.DPrintf(db.NETPROXYSRV, "Listen principal %v -> addr %v", ctx.Principal(), addr)
	l, err := nps.directListenFn(addr)
	// If Listen was unsuccessful, set the reply error appropriately
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error listen direct: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	}
	ep, err := constructEndpoint(true, nps.auth, nps.innerContainerIP, ctx.Principal().GetRealm(), l)
	if err != nil {
		db.DFatalf("Error construct endpoint: %v", err)
		return err
	}
	res.Endpoint = ep.GetProto()
	// Store the listener & assign it an ID
	lid := nps.addListener(l)
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
	ctx := c.(*Ctx)
	lid := Tlid(req.ListenerID)
	db.DPrintf(db.NETPROXYSRV, "Accept principal %v -> lid %v", ctx.Principal(), lid)
	l, ok := nps.getListener(lid)
	if !ok {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error accept unknown listener %v", lid)
		res.Err = sp.NewRerrorErr(fmt.Errorf("Unknown listener: %v", lid))
		return nil
	}
	proxyConn, err := l.Accept()
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error accept direct: %v", err)
		res.Err = sp.NewRerrorErr(fmt.Errorf("Error accept: %v", err))
		return nil
	}
	file, err := connToFile(proxyConn)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	// Link conn FD to context so that it stays in scope and doesn't get GC-ed
	// before it can be sent back to the client
	ctx.SetConn(file)
	// Set socket control message in output blob
	res.Blob.Iov[0] = constructSocketControlMsg(file)
	res.Err = sp.NewRerror()
	return nil
}

func (nps *NetProxySrvStubs) Close(c fs.CtxI, req netproto.CloseRequest, res *netproto.CloseResponse) error {
	// Set socket control message in output blob. Do this immediately to make
	// sure it is set, even if we return early
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{nil},
	}
	ctx := c.(*Ctx)
	lid := Tlid(req.ListenerID)
	db.DPrintf(db.NETPROXYSRV, "Close principal %v -> lid %v", ctx.Principal(), lid)
	ok := nps.delListener(lid)
	if !ok {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error close unknown listener %v", lid)
		res.Err = sp.NewRerrorErr(fmt.Errorf("Unknown listener: %v", lid))
		return nil
	}
	res.Err = sp.NewRerror()
	return nil
}

// Store a new listener, and assign it an ID
func (nps *NetProxySrvStubs) addListener(l net.Listener) Tlid {
	lid := Tlid(nps.lidctr.Add(1))

	nps.Lock()
	defer nps.Unlock()

	nps.listeners[lid] = l
	return lid
}

// Given an LID, retrieve the associated Listener
func (nps *NetProxySrvStubs) getListener(lid Tlid) (net.Listener, bool) {
	nps.Lock()
	defer nps.Unlock()

	l, ok := nps.listeners[lid]
	return l, ok
}

// Given an LID, retrieve the associated Listener
func (nps *NetProxySrvStubs) delListener(lid Tlid) bool {
	nps.Lock()
	defer nps.Unlock()

	l, ok := nps.listeners[lid]
	if ok {
		delete(nps.listeners, lid)
		l.Close()
	}
	return ok
}

func (npss *NetProxySrvStubs) ReportError(err error) {
	db.DPrintf(db.NETPROXYSRV_ERR, "ReportError err %v", err)
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

func constructEndpoint(verifyEndpoints bool, amgr auth.AuthMgr, ip sp.Tip, realm sp.Trealm, l net.Listener) (*sp.Tendpoint, error) {
	host, port, err := netsigma.QualifyAddrLocalIP(ip, l.Addr().String())
	if err != nil {
		db.DPrintf(db.ERROR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		return nil, err
	}
	ep := sp.NewEndpoint(sp.Taddrs{sp.NewTaddrRealm(host, sp.INNER_CONTAINER_IP, port, realm.String())}, realm)
	if verifyEndpoints && amgr == nil {
		db.DFatalf("Error construct endpoint without AuthMgr")
		return nil, fmt.Errorf("Try to construct endpoint without authsrv")
	}
	if amgr != nil {
		// Sign the endpoint
		if err := amgr.MintAndSetEndpointToken(ep); err != nil {
			db.DFatalf("Error sign endpoint: %v", err)
			return nil, err
		}
	}
	return ep, nil
}
