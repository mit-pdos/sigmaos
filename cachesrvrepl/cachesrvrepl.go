package cachesrvrepl

import (
	"sync"

	"google.golang.org/protobuf/proto"

	cacheproto "sigmaos/cache/proto"
	replproto "sigmaos/repl/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/replraft"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

//
// Replicated CacheSrv with same RPC interface (CacheSrv) has unreplicated CacheSrv
//

type CacheSrv struct {
	mu      sync.Mutex
	raftcfg *replraft.RaftConfig
	replSrv repl.Server
}

func NewCacheSrv(raftcfg *replraft.RaftConfig) *CacheSrv {
	cs := &CacheSrv{raftcfg: raftcfg}
	cs.replSrv = raftcfg.MakeServer()
	cs.replSrv.Start()
	db.DPrintf(db.ALWAYS, "%v: Starting repl server: %v", proc.GetName(), raftcfg)
	return cs
}

func newRequest(m proto.Message, cid sp.TclntId, s sp.Tseqno) (*replproto.ReplRequest, error) {
	b, err := proto.Marshal(m)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return &replproto.ReplRequest{Msg: b, ClntId: uint32(cid), Seqno: uint64(s)}, nil
}

func (cs *CacheSrv) CreateShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	db.DPrintf(db.CACHESRV_REPL, "CreateShard %v\n", req)

	replreq, err := newRequest(&req, req.TclntId(), req.Tseqno())
	if err != nil {
		return err
	}
	cs.replSrv.Process(replreq)
	return nil
}

func (cs *CacheSrv) DeleteShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV_REPL, "DeleteShard %v\n", req)
	return nil
}

func (cs *CacheSrv) FreezeShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV_REPL, "FreezeShard %v\n", req)
	return nil
}

func (cs *CacheSrv) DumpShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheDump) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV_REPL, "DumpShard %v\n", req)
	return nil
}

func (cs *CacheSrv) Put(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	db.DPrintf(db.CACHESRV_REPL, "Put %v", req)
	return nil
}

func (cs *CacheSrv) Get(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	db.DPrintf(db.CACHESRV_REPL, "Get %v", req)
	return nil
}

func (cs *CacheSrv) Delete(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	db.DPrintf(db.CACHESRV_REPL, "Delete %v", req)
	return nil
}
