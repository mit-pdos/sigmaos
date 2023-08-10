package cachesrvrepl

import (
	"sync"

	// "google.golang.org/protobuf/proto"

	//cacheproto "sigmaos/cache/proto"
	replproto "sigmaos/cache/replproto"
	// replproto "sigmaos/repl/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/replraft"
	// "sigmaos/serr"
	// sp "sigmaos/sigmap"
)

//
// Replicated CacheSrv with same RPC interface (CacheSrv) has unreplicated CacheSrv
//

type CacheSrvRepl struct {
	mu      sync.Mutex
	raftcfg *replraft.RaftConfig
	replSrv repl.Server
	rpcs    rpcsrv.RPCSrv
}

func NewCacheSrvRepl(raftcfg *replraft.RaftConfig, svci any) *CacheSrvRepl {
	cs := &CacheSrvRepl{raftcfg: raftcfg, rpcs: NewRPCSrv(svci, nil)}
	cs.replSrv = raftcfg.MakeServer(cs.applyOp)
	cs.replSrv.Start()
	db.DPrintf(db.ALWAYS, "%v: Starting repl server: %v %v", proc.GetName(), svci, raftcfg)
	return cs
}

func (cs *CacheSrvRepl) applyOp(req *replproto.ReplOpRequest, rep *replproto.ReplOpReply) error {
	db.DPrintf(db.CACHESRV_REPL, "ApplyOp %v\n", req)

	return nil
}

func (cs *CacheSrvRepl) SubmitOp(ctx fs.CtxI, req replproto.ReplOpRequest, rep *replproto.ReplOpReply) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	db.DPrintf(db.CACHESRV_REPL, "SubmitOp %v\n", req)
	if err := cs.replSrv.Process(&req, rep); err != nil {
		return err
	}
	return nil
}
