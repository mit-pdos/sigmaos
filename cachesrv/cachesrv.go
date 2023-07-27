package cachesrv

import (
	"errors"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	cacheproto "sigmaos/cache/proto"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessdevsrv"
	"sigmaos/sessp"
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

type cache struct {
	sync.Mutex
	cache map[string][]byte
}

type CacheSrv struct {
	shards []cache
	shrd   string
	tracer *tracing.Tracer
	perf   *perf.Perf
}

func RunCacheSrv(args []string) error {
	s := &CacheSrv{}
	if len(args) > 3 {
		s.shrd = args[3]
	}
	public, err := strconv.ParseBool(args[2])
	if err != nil {
		return err
	}
	s.shards = make([]cache, NSHARD)
	for i := 0; i < NSHARD; i++ {
		s.shards[i].cache = make(map[string][]byte)
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

	s.tracer = tracing.Init("cache", proc.GetSigmaJaegerIP())
	defer s.tracer.Flush()

	p, err := perf.MakePerf(perf.CACHESRV)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	s.perf = p
	defer s.perf.Done()

	return ssrv.RunServer()
}

func NewCacheSrv() *CacheSrv {
	s := &CacheSrv{shards: make([]cache, NSHARD)}
	for i := 0; i < NSHARD; i++ {
		s.shards[i].cache = make(map[string][]byte)
	}
	s.tracer = tracing.Init("cache", proc.GetSigmaJaegerIP())
	p, err := perf.MakePerf(perf.CACHESRV)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	s.perf = p
	return s
}

func (s *CacheSrv) ExitCacheSrv() {
	s.tracer.Flush()
	s.perf.Done()
}

// XXX support timeout
func (s *CacheSrv) Put(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := s.tracer.StartRPCSpan(&req, "Put")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Put %v\n", req)

	b := key2shard(req.Key)

	start := time.Now()
	s.shards[b].Lock()
	defer s.shards[b].Unlock()
	if time.Since(start) > 300*time.Microsecond {
		db.DPrintf(db.CACHE_LAT, "Long cache lock put: %v", time.Since(start))
	}

	s.shards[b].cache[req.Key] = req.Value
	return nil
}

func (s *CacheSrv) Get(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	e2e := time.Now()
	if false {
		_, span := s.tracer.StartRPCSpan(&req, "Get")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Get %v", req)
	b := key2shard(req.Key)

	start := time.Now()
	s.shards[b].Lock()
	defer s.shards[b].Unlock()
	if time.Since(start) > 300*time.Microsecond {
		db.DPrintf(db.CACHE_LAT, "Long cache lock get: %v", time.Since(start))
	}

	v, ok := s.shards[b].cache[req.Key]
	if ok {
		rep.Value = v
		return nil
	}
	if time.Since(e2e) > 1*time.Millisecond {
		db.DPrintf(db.CACHE_LAT, "Long e2e get: %v", time.Since(e2e))
	}
	return ErrMiss
}

func (s *CacheSrv) Delete(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := s.tracer.StartRPCSpan(&req, "Delete")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Delete %v", req)
	b := key2shard(req.Key)

	start := time.Now()
	s.shards[b].Lock()
	defer s.shards[b].Unlock()
	if time.Since(start) > 20*time.Millisecond {
		db.DPrintf(db.ALWAYS, "Time spent witing for cache lock: %v", time.Since(start))
	}

	_, ok := s.shards[b].cache[req.Key]
	if ok {
		delete(s.shards[b].cache, req.Key)
		return nil
	}
	return ErrMiss
}

type cacheSession struct {
	*inode.Inode
	shards []cache
	sid    sessp.Tsession
}

func (s *CacheSrv) mkSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.Inode, *serr.Err) {
	cs := &cacheSession{mfs.MakeDevInode(), s.shards, sid}
	db.DPrintf(db.CACHESRV, "mkSession %v %p\n", cs.shards, cs)
	return cs, nil
}

// XXX incremental read
func (cs *cacheSession) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	db.DPrintf(db.CACHESRV, "Dump cache %p %v\n", cs, cs.shards)
	m := make(map[string][]byte)
	for i, _ := range cs.shards {
		cs.shards[i].Lock()
		for k, v := range cs.shards[i].cache {
			m[k] = v
		}
		cs.shards[i].Unlock()
	}

	b, err := proto.Marshal(&cacheproto.CacheDump{Vals: m})
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return b, nil
}

func (cs *cacheSession) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	return 0, serr.MkErr(serr.TErrNotSupported, nil)
}

func (cs *cacheSession) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.CACHESRV, "Close %v\n", cs.sid)
	return nil
}
