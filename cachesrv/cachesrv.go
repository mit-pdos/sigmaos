package cachesrv

import (
	"errors"
	"hash/fnv"
	"strconv"
	"time"

	cacheproto "sigmaos/cache/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sessdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

const (
	DUMP   = "dump"
	NSHARD = 1009
)

var (
	ErrMiss = errors.New("cache miss")
)

func key2shard(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	bin := h.Sum32() % NSHARD
	return bin
}

type CacheSrv struct {
	shards []shard
	shrd   string
	tracer *tracing.Tracer
	perf   *perf.Perf
}

func RunCacheSrv(args []string) error {
	pn := ""
	if len(args) > 3 {
		pn = args[3]
	}
	public, err := strconv.ParseBool(args[2])
	if err != nil {
		return err
	}

	s := NewCacheSrv(pn)

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
	s.ExitCacheSrv()
	return nil
}

func NewCacheSrv(pn string) *CacheSrv {
	cs := &CacheSrv{shards: make([]shard, NSHARD)}
	for i := 0; i < NSHARD; i++ {
		cs.shards[i].cache = make(map[string][]byte)
	}
	cs.tracer = tracing.Init("cache", proc.GetSigmaJaegerIP())
	p, err := perf.MakePerf(perf.CACHESRV)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	cs.perf = p
	cs.shrd = pn
	return cs
}

func (cs *CacheSrv) ExitCacheSrv() {
	cs.tracer.Flush()
	cs.perf.Done()
}

func (cs *CacheSrv) Put(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Put")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Put %v\n", req)

	start := time.Now()

	b := key2shard(req.Key)
	s := cs.shards[b]
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

	b := key2shard(req.Key)
	s := cs.shards[b]
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

	b := key2shard(req.Key)
	s := cs.shards[b]
	ok := s.delete(req.Key)

	if time.Since(start) > 20*time.Millisecond {
		db.DPrintf(db.ALWAYS, "Time spent witing for cache lock: %v", time.Since(start))
	}

	if ok {
		return nil
	}
	return ErrMiss
}
