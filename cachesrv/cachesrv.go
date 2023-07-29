package cachesrv

import (
	"errors"
	"strconv"
	"sync"
	"time"

	cacheproto "sigmaos/cache/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

const (
	DUMP   = "dump"
	NSHARD = 1009 // for cached
)

var (
	ErrMiss = errors.New("cache miss")
)

type shardInfo struct {
	ready bool
	s     *shard
}

type shardMap map[uint32]*shardInfo

type CacheSrv struct {
	mu     sync.Mutex
	shards shardMap
	shrd   string
	tracer *tracing.Tracer
	perf   *perf.Perf
}

func RunCacheSrv(args []string, nshard uint32) error {
	pn := ""
	if len(args) > 3 {
		pn = args[3]
	}
	public, err := strconv.ParseBool(args[2])
	if err != nil {
		return err
	}

	s := NewCacheSrv(pn)

	for i := uint32(0); i < nshard; i++ {
		if err := s.createShard(i, true); err != nil {
			db.DFatalf("CreateShard %v\n", err)
		}
	}

	db.DPrintf(db.CACHESRV, "%v: Run %v\n", proc.GetName(), s.shrd)
	ssrv, err := sigmasrv.MakeSigmaSrvPublic(args[1]+s.shrd, s, db.CACHESRV, public)
	if err != nil {
		return err
	}
	if _, err := ssrv.Create(DUMP, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	if err := sessdevsrv.MkSessDev(ssrv.MemFs, DUMP, s.mkSession, nil); err != nil {
		return err
	}
	ssrv.RunServer()
	s.exitCacheSrv()
	return nil
}

func NewCacheSrv(pn string) *CacheSrv {
	cs := &CacheSrv{shards: make(map[uint32]*shardInfo)}
	cs.tracer = tracing.Init("cache", proc.GetSigmaJaegerIP())
	p, err := perf.MakePerf(perf.CACHESRV)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	cs.perf = p
	cs.shrd = pn
	return cs
}

func (cs *CacheSrv) exitCacheSrv() {
	cs.tracer.Flush()
	cs.perf.Done()
}

// XXX locking
func (cs *CacheSrv) lookupShard(s uint32) (*shard, bool) {
	sh, ok := cs.shards[s]
	if !sh.ready {
		return nil, false
	}
	return sh.s, ok
}

func (cs *CacheSrv) createShard(s uint32, b bool) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, ok := cs.shards[s]; ok {
		return serr.MkErr(serr.TErrExists, s)
	}
	cs.shards[s] = &shardInfo{ready: b, s: newShard()}
	return nil
}

func (cs *CacheSrv) CreateShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheOK) error {
	db.DPrintf(db.CACHESRV, "CreateShard %v\n", req)
	return cs.createShard(req.Shard, false)
}

func (cs *CacheSrv) FillShard(ctx fs.CtxI, req cacheproto.ShardFill, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "FillShard %v\n", req)

	if _, ok := cs.shards[req.Shard]; !ok {
		return serr.MkErr(serr.TErrNotfound, req.Shard)
	}
	cs.shards[req.Shard] = &shardInfo{s: newShard()}
	cs.shards[req.Shard].ready = true
	cs.shards[req.Shard].s.fill(req.Vals)
	return nil
}

func (cs *CacheSrv) DumpShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheDump) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "DumpShard %v\n", req)

	if _, ok := cs.shards[req.Shard]; !ok {
		return serr.MkErr(serr.TErrNotfound, req.Shard)
	}
	rep.Vals = cs.shards[req.Shard].s.dump()
	return nil
}

func (cs *CacheSrv) DeleteShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "DeleteShard %v\n", req)

	if _, ok := cs.shards[req.Shard]; !ok {
		return serr.MkErr(serr.TErrNotfound, req.Shard)
	}
	delete(cs.shards, req.Shard)
	return nil
}

// XXX support timeout
func (cs *CacheSrv) Put(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Put")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Put %v\n", req)

	start := time.Now()

	s, ok := cs.lookupShard(req.Shard)
	if !ok {
		return ErrMiss
	}
	err := s.put(req.Key, req.Value)

	if time.Since(start) > 300*time.Microsecond {
		db.DPrintf(db.CACHE_LAT, "Long cache lock put: %v", time.Since(start))
	}
	return err
}

func (cs *CacheSrv) Get(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	e2e := time.Now()
	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Get")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Get %v", req)
	start := time.Now()

	s, ok := cs.lookupShard(req.Shard)
	if !ok {
		return ErrMiss
	}
	v, ok := s.get(req.Key)

	if time.Since(start) > 300*time.Microsecond {
		db.DPrintf(db.CACHE_LAT, "Long cache lock get: %v", time.Since(start))
	}

	if ok {
		rep.Value = v
		return nil
	}
	if time.Since(e2e) > 1*time.Millisecond {
		db.DPrintf(db.CACHE_LAT, "Long e2e get: %v", time.Since(e2e))
	}
	return ErrMiss
}

func (cs *CacheSrv) Delete(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Delete")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Delete %v", req)

	start := time.Now()

	s, ok := cs.lookupShard(req.Shard)
	if !ok {
		return ErrMiss
	}
	ok = s.delete(req.Key)

	if time.Since(start) > 20*time.Millisecond {
		db.DPrintf(db.ALWAYS, "Time spent witing for cache lock: %v", time.Since(start))
	}

	if ok {
		return nil
	}
	return ErrMiss
}
