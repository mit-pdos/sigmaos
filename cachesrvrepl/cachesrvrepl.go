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
}

func NewCacheSrvRepl(raftcfg *replraft.RaftConfig) *CacheSrvRepl {
	cs := &CacheSrvRepl{raftcfg: raftcfg}
	cs.replSrv = raftcfg.MakeServer()
	cs.replSrv.Start()
	db.DPrintf(db.ALWAYS, "%v: Starting repl server: %v", proc.GetName(), raftcfg)
	return cs
}

// func newRequest(m proto.Message, cid sp.TclntId, s sp.Tseqno) (*replproto.ReplRequest, error) {
// 	b, err := proto.Marshal(m)
// 	if err != nil {
// 		return nil, serr.MkErrError(err)
// 	}
// 	return &replproto.ReplRequest{Msg: b, ClntId: uint32(cid), Seqno: uint64(s)}, nil
// }

func (cs *CacheSrvRepl) RequestOp(ctx fs.CtxI, req replproto.ReplOpRequest, rep *replproto.ReplOpReply) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	db.DPrintf(db.CACHESRV_REPL, "RequestOp %v\n", req)

	// replreq, err := newRequest(&req, req.TclntId(), req.Tseqno())
	// if err != nil {
	// 	return err
	// }
	// cs.replSrv.Process(replreq)
	return nil
}
