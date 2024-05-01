package netproxy

import (
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netproxy/proto"
	"sigmaos/proc"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/rpcclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type NetProxyClnt struct {
	sync.Mutex
	pe               *proc.ProcEnv
	canSignEndpoints bool
	verifyEndpoints  bool
	auth             auth.AuthMgr
	directDialFn     DialFn
	directListenFn   ListenFn
	seqcntr          *sessp.Tseqcntr
	trans            *NetProxyTrans
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
		directDialFn:     DialDirect,
		directListenFn:   ListenDirect,
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
		db.DPrintf(db.NETPROXYCLNT, "directDial %v", ep)
		c, err = npc.directDialFn(ep)
		db.DPrintf(db.NETPROXYCLNT, "directDial %v done ok:%v", ep, err == nil)
	}
	if err == nil {
		db.DPrintf(db.NETSIGMA_PERF, "Dial latency: %v", time.Since(start))
	}
	return c, err
}

func (npc *NetProxyClnt) Listen(addr *sp.Taddr) (*sp.Tendpoint, net.Listener, error) {
	var ep *sp.Tendpoint
	var l net.Listener
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
		l, err = npc.directListenFn(addr)
		if err != nil {
			db.DPrintf(db.NETPROXYCLNT_ERR, "Error directListen %v: %v", addr, err)
			return nil, nil, err
		}
		ep, err = constructEndpoint(npc.verifyEndpoints, npc.auth, npc.pe.GetInnerContainerIP(), npc.pe.GetRealm(), l)
		if err != nil {
			db.DPrintf(db.ERROR, "Error construct endpoint: %v", err)
			return nil, nil, err
		}
	}
	return ep, l, err
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
	conn, err := getNetproxydConn(npc.pe)
	if err != nil {
		return err
	}
	// Connect to the netproxy server
	trans := NewNetProxyTrans(conn, iovm)
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
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyDial request ep %v", npc.trans.conn, ep)
	// Endpoints should always have realms specified
	if ep.GetRealm() == sp.NOT_SET {
		db.DPrintf(db.ERROR, "Dial endpoint without realm set: %v", ep)
		return nil, fmt.Errorf("Realm not set")
	}
	if !ep.IsSigned() {
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
	return parseReturnedConn(res.Blob.Iov[0])
}

func (npc *NetProxyClnt) proxyListen(addr *sp.Taddr) (*sp.Tendpoint, net.Listener, error) {
	// Ensure that the connection to the netproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyListen request addr %v", npc.trans.conn, addr)
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
	return ep, NewListener(npc, Tlid(res.ListenerID), ep), nil
}

func (npc *NetProxyClnt) proxyAccept(lid Tlid) (net.Conn, error) {
	// Ensure that the connection to the netproxy server has been initialized
	if err := npc.init(); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error init netproxyclnt %v", err)
		return nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] proxyAccept request lip %v", npc.trans.conn, lid)
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
		return nil, err
	}
	db.DPrintf(db.NETPROXYCLNT, "proxyAccept response %v", res)
	// If an error occurred during dialing, bail out
	if res.Err.ErrCode != 0 {
		err := sp.NewErr(res.Err)
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error Accept: %v", err)
		return nil, err
	}
	return parseReturnedConn(res.Blob.Iov[0])
}

func (npc *NetProxyClnt) newListener(lid Tlid) net.Listener {
	db.DFatalf("Unimplemented")
	return nil
}

func (npc *NetProxyClnt) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
	c := NewProxyCall(sessp.NextSeqno(npc.seqcntr), iniov)
	rep, err := npc.dmx.SendReceive(c, outiov)
	if err != nil {
		return err
	} else {
		c := rep.(*ProxyCall)
		if len(outiov) != len(c.Iov) {
			return fmt.Errorf("netproxyclnt outiov len wrong: %v != %v", len(outiov), len(c.Iov))
		}
		return nil
	}
}
func (npc *NetProxyClnt) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	db.DPrintf(db.ERROR, "StatsSrv unimplemented")
	return nil, fmt.Errorf("Unimplemented")
}
