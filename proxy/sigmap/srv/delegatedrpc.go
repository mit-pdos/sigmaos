package srv

import (
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	sprpcchan "sigmaos/rpc/clnt/channel/spchannel"
	sessp "sigmaos/session/proto"
	sigmaclnt "sigmaos/sigmaclnt"
)

func (spp *SPProxySrv) runDelegatedRPC(sc *sigmaclnt.SigmaClnt, p *proc.Proc, rpcIdx uint64, pn string, iniov sessp.IoVec, outIOVSize uint64) {
	db.DPrintf(db.SPPROXYSRV, "[%v] Create clnt for delegated RPC(%v): %v", p.GetPid(), rpcIdx, pn)
	rpcchan, ok := spp.psm.GetRPCChannel(p.GetPid(), pn)
	// If we don't have a cached channel for this RPC target, create a new channel for it (and cache it)
	if !ok {
		db.DPrintf(db.SPPROXYSRV, "[%v] delegated RPC(%v) create new channel for: %v", p.GetPid(), rpcIdx, pn)
		if ep, ok := p.GetProcEnv().GetCachedEndpoint(pn); ok {
			var err error
			rpcchan, err = sprpcchan.NewSPChannelEndpoint(sc.FsLib, pn, ep)
			if err != nil {
				db.DPrintf(db.SPPROXYSRV_ERR, "Err create mounted RPC channel to run delegated RPCs (%v -> %v): %v", pn, ep, err)
				// TODO: remove fatal
				db.DFatalf("Err create mounted RPC channel to run delegated RPCs (%v -> %v): %v", pn, ep, err)
				return
			}
		} else {
			var err error
			rpcchan, err = sprpcchan.NewSPChannel(sc.FsLib, pn)
			if err != nil {
				db.DPrintf(db.SPPROXYSRV_ERR, "Err create unmounted RPC channel to run delegated RPCs (%v): %v", pn, err)
				// TODO: remove fatal
				db.DFatalf("Err create unmounted RPC channel to run delegated RPCs (%v): %v", pn, err)
				return
			}
		}
		// Cache the channel for later reuse
		spp.psm.PutRPCChannel(p.GetPid(), pn, rpcchan)
	} else {
		db.DPrintf(db.SPPROXYSRV, "[%v] delegated RPC(%v) reuse cached channel for: %v", rpcIdx, pn)
	}
	db.DPrintf(db.SPPROXYSRV, "[%v] Run delegated init RPC(%v)", p.GetPid(), rpcIdx)
	outiov := make(sessp.IoVec, outIOVSize)
	start := time.Now()
	err := rpcchan.SendReceive(iniov, outiov)
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "Err execute delegated RPC (%v): %v", pn, err)
		// TODO: remove fatal
		db.DFatalf("Err execute delegated RPC (%v): %v", pn, err)
	}
	db.DPrintf(db.SPPROXYSRV, "[%v] Done running delegated init RPC(%v)", p.GetPid(), rpcIdx)
	spp.psm.InsertReply(p, uint64(rpcIdx), outiov, err, start)
}

// Run delegated initialization RPCs
func (spp *SPProxySrv) runDelegatedInitializationRPCs(p *proc.Proc, sc *sigmaclnt.SigmaClnt) {
	// If the proc didn't ask for delegated initialization, bail out
	if !p.GetDelegateInit() {
		return
	}
	var wg sync.WaitGroup
	db.DPrintf(db.SPPROXYSRV, "[%v] Run delegated init RPCs", p.GetPid())
	for initRPCIdx, initRPC := range p.GetInitRPCs() {
		wg.Add(1)
		go func(initRPCIdx int, initRPC *proc.InitializationRPC) {
			defer wg.Done()
			db.DPrintf(db.ALWAYS, "initRPC IOV len: %v", len(initRPC.GetInputIOV()))
			spp.runDelegatedRPC(sc, p, uint64(initRPCIdx), initRPC.GetTargetPathname(), initRPC.GetInputIOV(), initRPC.GetNOutIOV())
		}(initRPCIdx, initRPC)
	}
	wg.Wait()
}
