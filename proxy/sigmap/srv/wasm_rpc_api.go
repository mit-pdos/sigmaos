package srv

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	sessp "sigmaos/session/proto"
	sigmaclnt "sigmaos/sigmaclnt"
	// wasmrt "sigmaos/proxy/wasm/rpc/wasmer"
)

type WASMRPCProxy struct {
	spp *SPProxySrv
	sc  *sigmaclnt.SigmaClnt
	p   *proc.Proc
}

func NewWASMRPCProxy(spp *SPProxySrv, sc *sigmaclnt.SigmaClnt, p *proc.Proc) *WASMRPCProxy {
	return &WASMRPCProxy{
		spp: spp,
		sc:  sc,
		p:   p,
	}
}

func (wp *WASMRPCProxy) Send(rpcIdx uint64, pn string, method string, b []byte, nOutIOV uint64) error {
	// Wrap the marshaled RPC byte slice in an RPC wrapper
	iniov, err := rpcclnt.WrapMarshaledRPCRequest("CacheSrv.MultiGet", sessp.IoVec{b})
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "Error wrap & marshal WASM-proxied RPC request: %v", err)
		return err
	}
	// Run the delegated RPC asynchronously
	go wp.spp.runDelegatedRPC(wp.sc, wp.p, rpcIdx, pn, iniov, nOutIOV)
	return nil
}

func (wp *WASMRPCProxy) Recv(rpcIdx uint64) ([]byte, error) {
	db.DFatalf("Unimplemented")
	return nil, nil
}
