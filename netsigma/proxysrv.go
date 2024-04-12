package netsigma

import (
	"fmt"
	"net"
	"os"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/netsigma/proto"
	"sigmaos/rpc"
	"sigmaos/rpcsrv"
	sp "sigmaos/sigmap"
)

type NetProxySrv struct {
	*NetProxySrvStubs
}

type NetProxySrvStubs struct {
	innerContainerIP sp.Tip
	auth             auth.AuthSrv
	directDialFn     DialFn
	directListenFn   ListenFn
	trans            *NetProxyRPCTrans
	rpcs             *rpcsrv.RPCSrv
}

func NewNetProxySrv(ip sp.Tip, as auth.AuthSrv) (*NetProxySrv, error) {
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
		&NetProxySrvStubs{
			innerContainerIP: ip,
			auth:             as,
			directDialFn:     DialDirect,
			directListenFn:   ListenDirect,
		},
	}
	rpcs := rpcsrv.NewRPCSrv(nps.NetProxySrvStubs, rpc.NewStatInfo())
	nps.rpcs = rpcs
	nps.trans = NewNetProxyRPCTrans(rpcs, socket)
	return nps, nil
}

func (nps *NetProxySrvStubs) Dial(ctx fs.CtxI, req proto.DialRequest, res *proto.DialResponse) error {
	mnt := sp.NewMountFromProto(req.GetMount())
	db.DPrintf(db.NETPROXYSRV, "Dial principal %v -> mnt %v", ctx.Principal(), mnt)
	// Verify the principal is authorized to establish the connection
	if _, err := nps.auth.MountIsAuthorized(ctx.Principal(), mnt); err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Dial unauthorized mount: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	}
	proxyConn, err := nps.directDialFn(mnt)
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
	// Get wrapper context in order to set output FD
	wctx := ctx.(*WrapperCtx)
	wctx.SetFile(file)
	return nil
}

func (nps *NetProxySrvStubs) Listen(ctx fs.CtxI, req proto.ListenRequest, res *proto.ListenResponse) error {
	db.DPrintf(db.NETPROXYSRV, "Listen principal %v", ctx.Principal())
	// Verify the principal is who they say they are
	if _, err := nps.auth.VerifyPrincipalIdentity(ctx.Principal()); err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Listen unable to verify principal identity: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	}
	proxyListener, err := nps.directListenFn(req.GetAddr())
	// If Dial was unsuccessful, set the reply error appropriately
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error listen direct: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	} else {
		res.Err = sp.NewRerror()
	}
	// Construct a mount for the listener
	mnt, err := constructMount(true, nps.auth, nps.innerContainerIP, ctx.Principal().GetRealm(), proxyListener)
	if err != nil {
		db.DFatalf("Error construct mount: %v", err)
		return err
	}
	res.Mount = mnt.GetProto()
	file, err := listenerToFile(proxyListener)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	// Get wrapper context in order to set output FD
	wctx := ctx.(*WrapperCtx)
	wctx.SetFile(file)
	return nil
}

func listenerToFile(proxyListener net.Listener) (*os.File, error) {
	f, err := proxyListener.(*net.TCPListener).File()
	if err != nil {
		db.DFatalf("Error get TCP listener fd: %v", err)
		return nil, err
	}
	// Return the file object for the socket
	return f, nil
}

func connToFile(proxyConn net.Conn) (*os.File, error) {
	f, err := proxyConn.(*net.TCPConn).File()
	if err != nil {
		db.DFatalf("Error get TCP conn fd: %v", err)
		return nil, err
	}
	// Return the file object for the socket
	return f, nil
}

func constructMount(verifyMounts bool, as auth.AuthSrv, ip sp.Tip, realm sp.Trealm, l net.Listener) (*sp.Tmount, error) {
	host, port, err := QualifyAddrLocalIP(ip, l.Addr().String())
	if err != nil {
		db.DPrintf(db.ERROR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		return nil, err
	}
	mnt := sp.NewMount(sp.Taddrs{sp.NewTaddrRealm(host, sp.INNER_CONTAINER_IP, port, realm.String())}, realm)
	if verifyMounts && as == nil {
		db.DFatalf("Error construct mount without AuthSrv")
		return nil, fmt.Errorf("Try to construct mount without authsrv")
	}
	if as != nil {
		// Sign the mount
		if err := as.MintAndSetMountToken(mnt); err != nil {
			db.DFatalf("Error sign mount: %v", err)
			return nil, err
		}
	}
	return mnt, nil
}

func (nps *NetProxySrv) Shutdown() {
	db.DPrintf(db.NETPROXYSRV, "Shutdown")
	os.Remove(sp.SIGMA_NETPROXY_SOCKET)
}
