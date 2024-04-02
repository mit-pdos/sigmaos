package netsigma

import (
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
		innerContainerIP: ip,
		auth:             as,
		directDialFn:     DialDirect,
		directListenFn:   ListenDirect,
	}
	rpcs := rpcsrv.NewRPCSrv(nps, rpc.NewStatInfo())
	nps.rpcs = rpcs
	nps.trans = NewNetProxyRPCTrans(rpcs, socket)
	return nps, nil
}

func (nps *NetProxySrv) Dial(ctx fs.CtxI, req proto.DialRequest, res *proto.DialResponse) error {
	mnt := sp.NewMountFromProto(req.GetMount())
	db.DPrintf(db.NETPROXYSRV, "Dial principal %v -> mnt %v", ctx.Principal(), mnt)
	proxyConn, err := nps.directDialFn(mnt)
	// If Dial was unsuccessful, set the reply error appropriately
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error dial direct: %v", err)
		res.Err = sp.NewRerrorErr(err)
		return nil
	} else {
		res.Err = sp.NewRerror()
	}
	fd, err := connToFD(proxyConn)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	// Get wrapper context in order to set output FD
	wctx := ctx.(*WrapperCtx)
	wctx.SetFD(fd)
	return nil
}

func (nps *NetProxySrv) Listen(ctx fs.CtxI, req proto.ListenRequest, res *proto.ListenResponse) error {
	db.DPrintf(db.NETPROXYSRV, "Listen principal %v", ctx.Principal())
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
	mnt, err := constructMount(nps.innerContainerIP, ctx.Principal().GetRealm(), proxyListener)
	if err != nil {
		db.DFatalf("Error construct mount: %v", err)
		return err
	}
	// Sign the mount
	if err := nps.auth.MintAndSetMountToken(mnt); err != nil {
		db.DFatalf("Error sign mount: %v", err)
		return err
	}
	res.Mount = mnt.GetProto()
	fd, err := listenerToFD(proxyListener)
	if err != nil {
		db.DFatalf("Error convert conn to FD: %v", err)
	}
	// Get wrapper context in order to set output FD
	wctx := ctx.(*WrapperCtx)
	wctx.SetFD(fd)
	return nil
}

func listenerToFD(proxyListener net.Listener) (int, error) {
	f, err := proxyListener.(*net.TCPListener).File()
	if err != nil {
		db.DFatalf("Error get TCP listener fd: %v", err)
		return 0, err
	}
	// Return the unix FD for the socket
	return int(f.Fd()), nil
}

func connToFD(proxyConn net.Conn) (int, error) {
	f, err := proxyConn.(*net.TCPConn).File()
	if err != nil {
		db.DFatalf("Error get TCP conn fd: %v", err)
		return 0, err
	}
	// Return the unix FD for the socket
	return int(f.Fd()), nil
}

func constructMount(ip sp.Tip, realm sp.Trealm, l net.Listener) (*sp.Tmount, error) {
	host, port, err := QualifyAddrLocalIP(ip, l.Addr().String())
	if err != nil {
		db.DPrintf(db.ERROR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Listen qualify local IP %v: %v", l.Addr().String(), err)
		return nil, err
	}
	return sp.NewMountService(sp.Taddrs{sp.NewTaddrRealm(host, sp.INNER_CONTAINER_IP, port, realm.String())}), nil
}

func (nps *NetProxySrv) Shutdown() {
	db.DPrintf(db.NETPROXYSRV, "Shutdown")
	os.Remove(sp.SIGMA_NETPROXY_SOCKET)
}
