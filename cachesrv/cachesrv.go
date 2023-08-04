package cachesrv

import (
	"fmt"
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

type Tstatus string

const (
	EMBRYO Tstatus = "Embryo"
	READY  Tstatus = "Ready"
	FROZEN Tstatus = "Frozen"
)

type shardInfo struct {
	status Tstatus
	s      *shard
}

type shardMap map[uint32]*shardInfo

type CacheSrv struct {
	mu        sync.Mutex
	shards    shardMap
	shrd      string
	tracer    *tracing.Tracer
	lastFence *sp.Tfence
	perf      *perf.Perf
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
		if err := s.createShard(i, READY); err != nil {
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
	cs := &CacheSrv{shards: make(map[uint32]*shardInfo), lastFence: sp.NullFence()}
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

//
// Fenced ops (with locking)
//

func (cs *CacheSrv) lookupShardFence(s uint32, f sp.Tfence) (*shard, error) {
	stale := cs.lastFence.LessThan(&f)
	if stale {
		db.DPrintf(db.CACHESRV, "New fence %v\n", f)
		cs.lastFence = &f
	}
	sh, ok := cs.shards[s]
	if !ok {
		if !stale {
			return nil, serr.MkErr(serr.TErrRetry, fmt.Sprintf("shard %d", s))
		} else {
			return nil, serr.MkErr(serr.TErrNotfound, fmt.Sprintf("shard %d", s))
		}
	}
	switch sh.status {
	case READY:
		return sh.s, nil
	case EMBRYO:
		return nil, serr.MkErr(serr.TErrRetry, fmt.Sprintf("shard %d", s))
	case FROZEN:
		return nil, serr.MkErr(serr.TErrStale, fmt.Sprintf("shard %d", s))
	default:
		db.DFatalf("lookupShardFence err status %v\n", sh.status)
		return nil, nil
	}
	return sh.s, nil
}

func (cs *CacheSrv) createShard(s uint32, status Tstatus) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, ok := cs.shards[s]; ok {
		return serr.MkErr(serr.TErrExists, s)
	}
	cs.shards[s] = &shardInfo{status: status, s: newShard()}
	return nil
}

func (cs *CacheSrv) CreateShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheOK) error {
	db.DPrintf(db.CACHESRV, "CreateShard %v\n", req)
	return cs.createShard(req.Shard, EMBRYO)
}

func (cs *CacheSrv) FreezeShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "FreezeShard %v\n", req)

	if si, ok := cs.shards[req.Shard]; !ok {
		return serr.MkErr(serr.TErrNotfound, req.Shard)
	} else {
		si.status = FROZEN
		return nil
	}
}

func (cs *CacheSrv) FillShard(ctx fs.CtxI, req cacheproto.ShardFill, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "FillShard %v\n", req)

	if si, ok := cs.shards[req.Shard]; !ok {
		return serr.MkErr(serr.TErrNotfound, req.Shard)
	} else if si.status == EMBRYO {
		si.s.fill(req.Vals)
		si.status = READY
		return nil
	} else {
		return serr.MkErr(serr.TErrStale, req.Shard)
	}
}

func (cs *CacheSrv) DumpShard(ctx fs.CtxI, req cacheproto.ShardArg, rep *cacheproto.CacheDump) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "DumpShard %v\n", req)

	if si, ok := cs.shards[req.Shard]; !ok {
		return serr.MkErr(serr.TErrNotfound, req.Shard)
	} else {
		rep.Vals = si.s.dump()
		return nil
	}
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

func (cs *CacheSrv) PutFence(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "PutFence %v\n", req)
	s, err := cs.lookupShardFence(req.Shard, req.Fence.Tfence())
	if err != nil {
		return err
	}
	if sp.Tmode(req.Mode) == sp.OAPPEND {
		err = s.append(req.Key, req.Value)
	} else {
		err = s.put(req.Key, req.Value)
	}
	return err
}

func (cs *CacheSrv) GetFence(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "GetFence %v\n", req)
	s, err := cs.lookupShardFence(req.Shard, req.Fence.Tfence())
	if err != nil {
		return err
	}
	v, ok := s.get(req.Key)
	if ok {
		rep.Value = v
		return nil
	}
	return serr.MkErr(serr.TErrNotfound, fmt.Sprintf("key %s", req.Key))
}

//
//  Unfenced ops and XXX locking
//

func (cs *CacheSrv) lookupShard(s uint32) (*shard, error) {
	sh, ok := cs.shards[s]
	if !ok {
		return nil, serr.MkErr(serr.TErrNotfound, fmt.Sprintf("shard %d", s))
	}
	if sh.status != READY {
		db.DFatalf("lookupShard err status %v\n", sh.status)
	}
	return sh.s, nil
}

func (cs *CacheSrv) Put(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if req.Fence.HasFence() {
		return cs.PutFence(ctx, req, rep)
	}

	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Put")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Put %v\n", req)

	start := time.Now()

	s, err := cs.lookupShard(req.Shard)

	if err != nil {
		return err
	}
	if sp.Tmode(req.Mode) == sp.OAPPEND {
		err = s.append(req.Key, req.Value)
	} else {
		err = s.put(req.Key, req.Value)
	}

	if time.Since(start) > 300*time.Microsecond {
		db.DPrintf(db.CACHE_LAT, "Long cache lock put: %v", time.Since(start))
	}
	return err
}

func (cs *CacheSrv) Get(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if req.Fence.HasFence() {
		return cs.GetFence(ctx, req, rep)
	}

	e2e := time.Now()
	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Get")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Get %v", req)
	start := time.Now()

	s, err := cs.lookupShard(req.Shard)
	if err != nil {
		return err
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
	return serr.MkErr(serr.TErrNotfound, fmt.Sprintf("key %s", req.Key))
}

func (cs *CacheSrv) Delete(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Delete")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Delete %v", req)

	start := time.Now()

	s, err := cs.lookupShard(req.Shard)
	if err != nil {
		return err
	}
	ok := s.delete(req.Key)

	if time.Since(start) > 20*time.Millisecond {
		db.DPrintf(db.ALWAYS, "Time spent witing for cache lock: %v", time.Since(start))
	}

	if ok {
		return nil
	}
	return serr.MkErr(serr.TErrNotfound, req.Key)
}
