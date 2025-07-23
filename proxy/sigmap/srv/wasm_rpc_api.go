package srv

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sigmaclnt "sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
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
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Error wrap & marshal WASM-proxied RPC request: %v", wp.p.GetPid(), err)
		return err
	}
	// Run the delegated RPC asynchronously
	go wp.spp.runDelegatedRPC(wp.sc, wp.p, rpcIdx, pn, iniov, nOutIOV)
	return nil
}

func (wp *WASMRPCProxy) Recv(rpcIdx uint64) ([]byte, error) {
	outiov, err := wp.spp.psm.GetReply(wp.p.GetPid(), rpcIdx)
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Err GetReply(%v) in WasmRPCRecv: %v", wp.p.GetPid(), err)
		return nil, err
	}
	rep := &rpcproto.Rep{}
	if err := proto.Unmarshal(outiov[0], rep); err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Err Unmarshal(%v) in WasmRPCRecv: %v", wp.p.GetPid(), err)
		return nil, serr.NewErrError(err)
	}
	if rep.Err.ErrCode != 0 {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Err ErrCode(%v) in WasmRPCRecv: %v", wp.p.GetPid(), rep.Err.ErrCode)
		return nil, sp.NewErr(rep.Err)
	}
	if len(outiov) != 2 {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Err RPC(%v) unexpected outiov len: %v", wp.p.GetPid(), len(outiov))
		return nil, serr.NewErrError(fmt.Errorf("Err unexpected outiov len: %v", len(outiov)))
	}
	return outiov[1], nil
}
