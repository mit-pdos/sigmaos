package srv

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/proc"
	wasmrpc "sigmaos/proxy/wasm/rpc"
	rpcclnt "sigmaos/rpc/clnt"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sigmaclnt "sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type WASMRPCProxy struct {
	spp *SPProxySrv
	sc  *sigmaclnt.SigmaClnt
	p   *proc.Proc
}

func NewWASMRPCProxy(spp *SPProxySrv, sc *sigmaclnt.SigmaClnt, p *proc.Proc) wasmrpc.RPCAPI {
	return &WASMRPCProxy{
		spp: spp,
		sc:  sc,
		p:   p,
	}
}

func (wp *WASMRPCProxy) Send(rpcIdx uint64, pn string, method string, b []byte, nOutIOV uint64) error {
	// Copy the data, because the shared buffer pointed to by b may be
	// overwritten by the next asynchronous RPC
	reqBytes := make([]byte, len(b))
	copy(reqBytes, b)
	// Wrap the marshaled RPC byte slice in an RPC wrapper
	iniov, err := rpcclnt.WrapMarshaledRPCRequest(method, sessp.IoVec{reqBytes})
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Error wrap & marshal WASM-proxied RPC request: %v", wp.p.GetPid(), err)
		return err
	}
	// Run the delegated RPC asynchronously, and add an extra output IOVec slot
	// for the RPC wrapper
	go wp.spp.runDelegatedRPC(wp.sc, wp.p, rpcIdx, pn, iniov, nOutIOV+1)
	return nil
}

func (wp *WASMRPCProxy) Recv(rpcIdx uint64) ([]byte, error) {
	outiov, err := wp.spp.psm.GetReply(wp.p.GetPid(), rpcIdx)
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Err GetReply(%v) in WasmRPCRecv: %v", wp.p.GetPid(), err)
		return nil, err
	}
	// Remove the RPC wrapper
	rep := &rpcproto.Rep{}
	if err := proto.Unmarshal(outiov[0], rep); err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Err Unmarshal(%v) in WasmRPCRecv: %v", wp.p.GetPid(), err)
		return nil, serr.NewErrError(err)
	}
	if rep.Err.ErrCode != 0 {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Err ErrCode(%v) in WasmRPCRecv: %v", wp.p.GetPid(), rep.Err.ErrCode)
		return nil, sp.NewErr(rep.Err)
	}
	// We don't handle blobs right now, so we only expect 2 out IOVecs
	if len(outiov) != 2 {
		db.DPrintf(db.SPPROXYSRV_ERR, "[%v] Err RPC(%v) unexpected outiov len: %v", wp.p.GetPid(), len(outiov))
		return nil, serr.NewErrError(fmt.Errorf("Err unexpected outiov len: %v", len(outiov)))
	}
	return outiov[1], nil
}
