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
	"sigmaos/protdevsrv"
	"sigmaos/serr"
	"sigmaos/sessdevsrv"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/tracing"
)

const (
	DUMP = "dump"
	NBIN = 1009
)

var (
	ErrMiss = errors.New("cache miss")
)

func key2bin(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	bin := h.Sum32() % NBIN
	return bin
}

type cache struct {
	sync.Mutex
	cache map[string][]byte
}

type CacheSrv struct {
	bins   []cache
	shrd   string
	tracer *tracing.Tracer
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
	s.bins = make([]cache, NBIN)
	for i := 0; i < NBIN; i++ {
		s.bins[i].cache = make(map[string][]byte)
	}
	db.DPrintf(db.CACHESRV, "%v: Run %v\n", proc.GetName(), s.shrd)
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.CACHE+s.shrd, s, public)
	if err != nil {
		return err
	}
	if err := sessdevsrv.MkSessDev(pds.MemFs, DUMP, s.mkSession, nil); err != nil {
		return err
	}

	s.tracer = tracing.Init("cache", proc.GetSigmaJaegerIP())
	defer s.tracer.Flush()

	p, err := perf.MakePerf(perf.CACHESRV)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	return pds.RunServer()
}

// XXX support timeout
func (s *CacheSrv) Put(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := s.tracer.StartRPCSpan(&req, "Put")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Put %v\n", req)

	b := key2bin(req.Key)

	start := time.Now()
	s.bins[b].Lock()
	defer s.bins[b].Unlock()
	if time.Since(start) > 20*time.Millisecond {
		db.DPrintf(db.ALWAYS, "Time spent witing for cache lock: %v", time.Since(start))
	}

	s.bins[b].cache[req.Key] = req.Value
	return nil
}

func (s *CacheSrv) Get(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := s.tracer.StartRPCSpan(&req, "Get")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Get %v", req)
	b := key2bin(req.Key)

	start := time.Now()
	s.bins[b].Lock()
	defer s.bins[b].Unlock()
	if time.Since(start) > 20*time.Millisecond {
		db.DPrintf(db.ALWAYS, "Time spent witing for cache lock: %v", time.Since(start))
	}

	v, ok := s.bins[b].cache[req.Key]
	if ok {
		rep.Value = v
		return nil
	}
	return ErrMiss
}

func (s *CacheSrv) Delete(ctx fs.CtxI, req cacheproto.CacheRequest, rep *cacheproto.CacheResult) error {
	if false {
		_, span := s.tracer.StartRPCSpan(&req, "Delete")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Delete %v", req)
	b := key2bin(req.Key)

	start := time.Now()
	s.bins[b].Lock()
	defer s.bins[b].Unlock()
	if time.Since(start) > 20*time.Millisecond {
		db.DPrintf(db.ALWAYS, "Time spent witing for cache lock: %v", time.Since(start))
	}

	_, ok := s.bins[b].cache[req.Key]
	if ok {
		delete(s.bins[b].cache, req.Key)
		return nil
	}
	return ErrMiss
}

type cacheSession struct {
	*inode.Inode
	bins []cache
	sid  sessp.Tsession
}

func (s *CacheSrv) mkSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.Inode, *serr.Err) {
	cs := &cacheSession{mfs.MakeDevInode(), s.bins, sid}
	db.DPrintf(db.CACHESRV, "mkSession %v %p\n", cs.bins, cs)
	return cs, nil
}

// XXX incremental read
func (cs *cacheSession) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	db.DPrintf(db.CACHESRV, "Dump cache %p %v\n", cs, cs.bins)
	m := make(map[string][]byte)
	for i, _ := range cs.bins {
		cs.bins[i].Lock()
		for k, v := range cs.bins[i].cache {
			m[k] = v
		}
		cs.bins[i].Unlock()
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
