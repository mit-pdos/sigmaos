package srv

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	sessp "sigmaos/session/proto"
	sigmaclnt "sigmaos/sigmaclnt"
)

func (spp *SPProxySrv) runDelegatedRPC(sc *sigmaclnt.SigmaClnt, p *proc.Proc, rpcIdx uint64, pn string, iniov *sessp.IoVec, outIOVSize uint64) {
	db.DPrintf(db.SPPROXYSRV, "[%v] Get RPCChannel for delegated RPC(%v): %v", p.GetPid(), rpcIdx, pn)
	rpcchan, err := spp.psm.GetRPCChannel(sc, p.GetPid(), rpcIdx, pn)
	if err != nil {
		// TODO: handle error gracefully
		db.DFatalf("[%v] Err make delegated RPC(%v) channel pn:%v err:%v", p.GetPid(), rpcIdx, pn, err)
	}
	// If we don't have a cached channel for this RPC target, create a new channel for it (and cache it)
	db.DPrintf(db.SPPROXYSRV, "[%v] Run delegated init RPC(%v)", p.GetPid(), rpcIdx)
	outiov := sessp.NewUnallocatedIoVec(int(outIOVSize), nil)
	start := time.Now()
	if err := rpcchan.SendReceive(iniov, outiov); err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "Err execute delegated RPC (%v): %v", pn, err)
		// TODO: remove fatal
		db.DFatalf("Err execute delegated RPC (%v): %v", pn, err)
	}
	db.DPrintf(db.SPPROXYSRV, "[%v] Done running delegated init RPC(%v)", p.GetPid(), rpcIdx)
	spp.psm.InsertReply(p, uint64(rpcIdx), outiov, err, start)
}
