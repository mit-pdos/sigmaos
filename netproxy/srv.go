package netproxy

import (
	"fmt"
	"net"
	"os"

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
	conn             *net.UnixConn
	innerContainerIP sp.Tip
	auth             auth.AuthMgr
	directDialFn     DialFn
	directListenFn   ListenFn
	rpcs             *rpcsrv.RPCSrv
	dmx              *demux.DemuxSrv
	ctx              fs.CtxI
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
		auth:             nps.auth,
		conn:             conn,
		innerContainerIP: nps.innerContainerIP,
		directDialFn:     DialDirect,
		directListenFn:   ListenDirect,
		ctx:              ctx.NewPrincipalOnlyCtx(p),
	}
	npss.rpcs = rpcsrv.NewRPCSrv(npss, rpc.NewStatInfo())
	// Start a demux server to handle requests & concurrency
	npss.dmx = demux.NewDemuxSrv(npss, NewNetProxyTrans(conn, demux.NewIoVecMap()))
}

func (npss *NetProxySrvStubs) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	db.DPrintf(db.NETPROXYSRV, "ServeRequest: %v", c)
	req := c.(*ProxyCall)
	rep, err := npss.rpcs.WriteRead(npss.ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV, "ServeRequest: writeRead err %v", err)
	}
	return NewProxyCall(req.Seqno, rep, true), nil
}

func (nps *NetProxySrvStubs) Dial(ctx fs.CtxI, req netproto.DialRequest, res *netproto.DialResponse) error {
	ep := sp.NewEndpointFromProto(req.GetEndpoint())
	db.DPrintf(db.NETPROXYSRV, "Dial principal %v -> ep %v", ctx.Principal(), ep)
	// Verify the principal is authorized to establish the connection
	if _, err := nps.auth.EndpointIsAuthorized(ctx.Principal(), ep); err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Dial unauthorized endpoint: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	}
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
	// Set socket control message in output blob
	res.Blob = &rpcproto.Blob{
		Iov: [][]byte{constructSocketControlMsg(file)},
	}
	return nil
}

//func (nps *NetProxySrvStubs) Listen(ctx fs.CtxI, req netproto.ListenRequest, res *netproto.ListenResponse) error {
//	db.DPrintf(db.NETPROXYSRV, "Listen principal %v", ctx.Principal())
//	// Verify the principal is who they say they are
//	if _, err := nps.auth.VerifyPrincipalIdentity(ctx.Principal()); err != nil {
//		db.DPrintf(db.NETPROXYSRV_ERR, "Error Listen unable to verify principal identity: %v", err)
//		res.Err = sp.NewRerrorErr(err)
//		return nil
//	}
//	proxyListener, err := nps.directListenFn(req.GetAddr())
//	// If Dial was unsuccessful, set the reply error appropriately
//	if err != nil {
//		db.DPrintf(db.NETPROXYSRV_ERR, "Error listen direct: %v", err)
//		res.Err = sp.NewRerrorErr(err)
//		return nil
//	} else {
//		res.Err = sp.NewRerror()
//	}
//	// Construct a endpoint for the listener
//	ep, err := constructEndpoint(true, nps.auth, nps.innerContainerIP, ctx.Principal().GetRealm(), proxyListener)
//	if err != nil {
//		db.DFatalf("Error construct endpoint: %v", err)
//		return err
//	}
//	res.Endpoint = ep.GetProto()
//	file, err := listenerToFile(proxyListener)
//	if err != nil {
//		db.DFatalf("Error convert conn to FD: %v", err)
//	}
//	// Get wrapper context in order to set output FD
//	wctx := ctx.(*WrapperCtx)
//	//	wctx.SetFile(file)
//	return nil
//}

func (npss *NetProxySrvStubs) ReportError(err error) {
	db.DPrintf(db.NETPROXYSRV_ERR, "ReportError err %v", err)
	db.DPrintf(db.NETPROXYSRV, "Close conn principal %v", npss.ctx.Principal())
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
