package srv

import (
	"fmt"
	"sort"
	"sync"
	"time"

	cacheproto "sigmaos/apps/cache/proto"

	"sigmaos/api/fs"
	"sigmaos/apps/cache"
	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	rpcdevsrv "sigmaos/rpc/dev/srv"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
	"sigmaos/util/tracing"
)

type Tstatus string

const (
	READY                    Tstatus = "Ready"
	FROZEN                   Tstatus = "Frozen"
	SHARD_STAT_SCAN_INTERVAL         = 2 * time.Second
)

type shardInfo struct {
	status Tstatus
	s      *shard
}

type shardMap map[cache.Tshard]*shardInfo

type CacheSrv struct {
	mu         sync.Mutex
	shards     shardMap
	shrd       string
	tracer     *tracing.Tracer
	lastFence  *sp.Tfence
	ssrv       *sigmasrv.SigmaSrv
	perf       *perf.Perf
	shardStats []shardStats
}

func RunCacheSrv(args []string, nshard int, useEPCache bool) error {
	pn := ""
	if len(args) > 2 {
		pn = args[2]
	}

	pe := proc.GetProcEnv()
	s, err := NewCacheSrv(pe, args[1], pn, nshard, useEPCache)
	if err != nil {
		return err
	}

	s.ssrv.RunServer()
	s.exitCacheSrv()
	return nil
}

func (cs *CacheSrv) manageShardHitCnts() {
	for {
		time.Sleep(SHARD_STAT_SCAN_INTERVAL)
		// Copy the list of shards
		cs.mu.Lock()
		// If the number of shards has changed, re-create the shardStats slice.
		// Otherwise, reuse it to avoid allocating while holding the lock.
		if len(cs.shardStats) != len(cs.shards) {
			cs.shardStats = make([]shardStats, len(cs.shards))
		}
		idx := 0
		for sid, si := range cs.shards {
			cs.shardStats[idx].shardID = sid
			// Reset the hit count on each shard, and note the old hit count
			cs.shardStats[idx].hitCnt = si.s.resetHitCnt()
			idx++
		}
		cs.mu.Unlock()
		// Sort the shards by hit count
		sort.Slice(cs.shardStats, func(i, j int) bool {
			return cs.shardStats[i].hitCnt < cs.shardStats[j].hitCnt
		})
		db.DPrintf(db.CACHESRV, "Shard stats: %v", cs.shardStats)
	}
}

func NewCacheSrv(pe *proc.ProcEnv, dirname string, pn string, nshard int, useEPCache bool) (*CacheSrv, error) {
	cs := &CacheSrv{shards: make(map[cache.Tshard]*shardInfo), lastFence: sp.NullFence()}
	p, err := perf.NewPerf(pe, perf.CACHESRV)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	cs.perf = p
	cs.shrd = pn
	for i := 0; i < nshard; i++ {
		if err := cs.createShard(cache.Tshard(i), sp.NoFence(), make(cache.Tcache)); err != nil {
			db.DFatalf("CreateShard %v\n", err)
		}
	}
	db.DPrintf(db.CACHESRV, "Run %v %v", dirname, cs.shrd)
	var ssrv *sigmasrv.SigmaSrv
	svcInstanceName := dirname + cs.shrd
	// If not using EPCache, post EP in the realm namespace
	if !useEPCache {
		ssrv, err = sigmasrv.NewSigmaSrv(svcInstanceName, cs, pe)
	} else {
		// Otherwise, don't post EP (and instead post EP in the EP cache service)
		ssrv, err = sigmasrv.NewSigmaSrv("", cs, pe)
		start := time.Now()
		if epcsrvEP, ok := pe.GetCachedEndpoint(epcache.EPCACHE); ok {
			if err := epcacheclnt.MountEPCacheSrv(ssrv.MemFs.SigmaClnt().FsLib, epcsrvEP); err != nil {
				db.DFatalf("Err mount epcache srv: %v", err)
			}
		}
		perf.LogSpawnLatency("cachesrv.MountEPCacheSrv", pe.GetPID(), pe.GetSpawnTime(), start)
		start = time.Now()
		epcc, err := epcacheclnt.NewEndpointCacheClnt(ssrv.MemFs.SigmaClnt().FsLib)
		if err != nil {
			db.DFatalf("Err EPCC: %v", err)
		}
		perf.LogSpawnLatency("cachesrv.NewEPCacheClnt", pe.GetPID(), pe.GetSpawnTime(), start)
		start = time.Now()
		ep := ssrv.MemFs.GetSigmaPSrvEndpoint()
		if err := epcc.RegisterEndpoint(svcInstanceName, pe.GetPID().String(), ep); err != nil {
			db.DFatalf("Err RegisterEP: %v", err)
		}
		perf.LogSpawnLatency("EPCacheSrv.RegisterEP", pe.GetPID(), pe.GetSpawnTime(), start)
	}
	if err != nil {
		return nil, err
	}
	if _, err := ssrv.Create(cache.DUMP, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return nil, err
	}
	if err := rpcdevsrv.NewSessDev(ssrv.MemFs, cache.DUMP, cs.newSession, nil); err != nil {
		return nil, err
	}
	cs.ssrv = ssrv
	go cs.manageShardHitCnts()
	return cs, nil
}

func (cs *CacheSrv) exitCacheSrv() {
	//	cs.tracer.Flush()
	cs.perf.Done()
}

//
// Fenced ops (with locking)
//

func (cs *CacheSrv) cmpFence(f sp.Tfence) sp.Tfencecmp {
	if !f.HasFence() {
		// cached runs without fence
		db.DPrintf(db.FENCEFS, "no fence %v\n", f)
	}
	if !cs.lastFence.IsInitialized() {
		db.DPrintf(db.FENCEFS, "initialize fence %v\n", f)
		cs.lastFence.Upgrade(&f)
		return sp.FENCE_EQ
	}
	return cs.lastFence.Cmp(&f)
}

func (cs *CacheSrv) cmpFenceUpgrade(f sp.Tfence) sp.Tfencecmp {
	cmp := cs.cmpFence(f)
	if cmp == sp.FENCE_LT {
		db.DPrintf(db.FENCEFS, "New fence %v\n", f)
		cs.lastFence.Upgrade(&f)
	}
	return cmp
}

// For Put and Get
func (cs *CacheSrv) lookupShardFence(s cache.Tshard, f sp.Tfence) (*shardInfo, error) {
	cmp := cs.cmpFence(f)
	if cmp == sp.FENCE_LT {
		// cs is behind let the client retry until cs catches up
		db.DPrintf(db.ALWAYS, "f %v shard %v cs behind; retry\n", cs.lastFence, s)
		return nil, serr.NewErr(serr.TErrRetry, fmt.Sprintf("shard %v", s))
	}
	sh, ok := cs.shards[s]
	if !ok {
		// if client is behind, return stale
		if cmp == sp.FENCE_GT {
			return nil, serr.NewErr(serr.TErrStale, fmt.Sprintf("shard %v", s))
		}
		// cs and client are in same config but server hasn't received
		// the shard yet.  let the client retry until the server
		// catchup first.
		db.DPrintf(db.ALWAYS, "f %v shard %v cs waiting for shard; retry\n", cs.lastFence, s)
		return nil, serr.NewErr(serr.TErrRetry, fmt.Sprintf("shard %v", s))
	}
	switch sh.status {
	case READY:
		return sh, nil
	case FROZEN:
		return nil, serr.NewErr(serr.TErrStale, fmt.Sprintf("shard %v", s))
	default:
		db.DFatalf("lookupShardFence err status %v\n", sh.status)
		return nil, nil
	}
	return sh, nil
}

func (cs *CacheSrv) loadShard(s cache.Tshard, vals cache.Tcache) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, ok := cs.shards[s]; !ok {
		return serr.NewErr(serr.TErrNotfound, s)
	}
	cs.shards[s].s.fill(vals)
	return nil
}

func (cs *CacheSrv) createShard(s cache.Tshard, f sp.Tfence, vals cache.Tcache) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cmp := cs.cmpFenceUpgrade(f); cmp == sp.FENCE_GT {
		return serr.NewErr(serr.TErrStale, fmt.Sprintf("shard %v", s))
	}
	if _, ok := cs.shards[s]; ok {
		return serr.NewErr(serr.TErrExists, s)
	}
	sh := newShard()
	sh.fill(vals)
	cs.shards[s] = &shardInfo{status: READY, s: sh}
	return nil
}

func (cs *CacheSrv) CreateShard(ctx fs.CtxI, req cacheproto.ShardReq, rep *cacheproto.CacheOK) error {
	db.DPrintf(db.CACHESRV, "CreateShard %v\n", req)
	return cs.createShard(req.Tshard(), req.Fence.Tfence(), req.Vals)
}

func (cs *CacheSrv) DeleteShard(ctx fs.CtxI, req cacheproto.ShardReq, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "DeleteShard %v\n", req)

	if _, ok := cs.shards[req.Tshard()]; !ok {
		return serr.NewErr(serr.TErrNotfound, req.Shard)
	}
	delete(cs.shards, req.Tshard())
	return nil
}

func (cs *CacheSrv) FreezeShard(ctx fs.CtxI, req cacheproto.ShardReq, rep *cacheproto.CacheOK) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "FreezeShard %v\n", req)

	if cmp := cs.cmpFenceUpgrade(req.Fence.Tfence()); cmp == sp.FENCE_GT {
		return serr.NewErr(serr.TErrStale, fmt.Sprintf("shard %v", req.Tshard()))
	}
	si, ok := cs.shards[req.Tshard()]
	if !ok {
		return serr.NewErr(serr.TErrNotfound, req.Tshard())
	}
	switch si.status {
	case READY:
		si.status = FROZEN
	case FROZEN:
		db.DPrintf(db.ALWAYS, "f %v %v already frozen\n", cs.lastFence, req.Tshard())
	}
	return nil
}

func (cs *CacheSrv) DumpShard(ctx fs.CtxI, req cacheproto.ShardReq, rep *cacheproto.ShardData) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "DumpShard %v\n", req)

	if cmp := cs.cmpFence(req.Fence.Tfence()); cmp == sp.FENCE_GT {
		db.DPrintf(db.CACHESRV_ERR, "DumpShard(%v) err stale fence", req.Tshard())
		return serr.NewErr(serr.TErrStale, fmt.Sprintf("shard %v", req.Tshard()))
	}
	if si, ok := cs.shards[req.Tshard()]; !ok {
		db.DPrintf(db.CACHESRV_ERR, "DumpShard(%v) err not found", req.Tshard())
		return serr.NewErr(serr.TErrNotfound, req.Tshard())
	} else {
		rep.Vals = si.s.dump()
	}
	db.DPrintf(db.CACHESRV, "DumpShard(%v) done", req.Tshard())
	return nil
}

func (cs *CacheSrv) PutFence(ctx fs.CtxI, req cacheproto.CacheReq, rep *cacheproto.CacheRep) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "PutFence %v\n", req)
	si, err := cs.lookupShardFence(req.Tshard(), req.Fence.Tfence())
	if err != nil {
		return err
	}
	if sp.Tmode(req.Mode) == sp.OAPPEND {
		err = si.s.append(req.Key, req.Value)
	} else {
		err = si.s.put(req.Key, req.Value)
	}
	return err
}

func (cs *CacheSrv) GetFence(ctx fs.CtxI, req cacheproto.CacheReq, rep *cacheproto.CacheRep) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	db.DPrintf(db.CACHESRV, "GetFence %v\n", req)
	si, err := cs.lookupShardFence(req.Tshard(), req.Fence.Tfence())
	if err != nil {
		return err
	}
	v, ok := si.s.get(req.Key)
	if ok {
		rep.Value = v
		return nil
	}
	return serr.NewErr(serr.TErrNotfound, fmt.Sprintf("key %s", req.Key))
}

//
//  Unfenced ops and XXX locking
//

func (cs *CacheSrv) lookupShard(s cache.Tshard) (*shard, error) {
	sh, ok := cs.shards[s]
	if !ok {
		return nil, serr.NewErr(serr.TErrNotfound, fmt.Sprintf("shard %v", s))
	}
	if sh.status != READY {
		db.DFatalf("lookupShard %v err status %v\n", s, sh.status)
	}
	return sh.s, nil
}

func (cs *CacheSrv) Put(ctx fs.CtxI, req cacheproto.CacheReq, rep *cacheproto.CacheRep) error {
	if req.Fence.HasFence() {
		return cs.PutFence(ctx, req, rep)
	}

	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Put")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Put %v\n", req)

	start := time.Now()

	s, err := cs.lookupShard(req.Tshard())

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

// Return the IDs of the topN hottest shards
func (cs *CacheSrv) GetHotShards(ctx fs.CtxI, req cacheproto.HotShardsReq, rep *cacheproto.HotShardsRep) error {
	rep.ShardIDs = make([]uint32, 0, req.TopN)
	rep.HitCnts = make([]uint64, 0, req.TopN)
	defer func() {
		db.DPrintf(db.CACHESRV, "HotShards: %v", rep.ShardIDs)
	}()

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// If we don't have any shard stats yet, bail out
	if len(cs.shardStats) == 0 {
		return nil
	}
	hottestIdx := len(cs.shardStats) - 1
	for i := 0; i < int(req.TopN) && i <= hottestIdx; i++ {
		rep.ShardIDs = append(rep.ShardIDs, uint32(cs.shardStats[hottestIdx-i].shardID))
		rep.HitCnts = append(rep.HitCnts, cs.shardStats[hottestIdx-i].hitCnt)
	}
	return nil
}

func (cs *CacheSrv) Get(ctx fs.CtxI, req cacheproto.CacheReq, rep *cacheproto.CacheRep) error {
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

	s, err := cs.lookupShard(req.Tshard())
	if err != nil {
		db.DPrintf(db.CACHESRV, "lookupShard error %v", err)
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
	db.DPrintf(db.CACHESRV, "Get %v key not found", req)
	return serr.NewErr(serr.TErrNotfound, fmt.Sprintf("key %s", req.Key))
}

func (cs *CacheSrv) MultiGet(ctx fs.CtxI, req cacheproto.CacheMultiGetReq, rep *cacheproto.CacheMultiGetRep) error {
	if req.Fence.HasFence() {
		// TODO: implement fenced multi-get
		db.DFatalf("Fenced multi-get unimplemented")
		// return cs.MultiGetFence(ctx, req, rep)
	}

	db.DPrintf(db.CACHESRV, "MultiGet %v", req)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	rep.Blob = &rpcproto.Blob{}

	bufs := make([][]byte, 0, len(req.Gets))
	totalLength := 0
	for _, getReq := range req.Gets {
		s, err := cs.lookupShard(getReq.Tshard())
		if err != nil {
			db.DPrintf(db.CACHESRV, "lookupShard error %v", err)
			return err
		}
		v, ok := s.get(getReq.Key)
		if ok {
			// Append value to blob & continue processing gets
			bufs = append(bufs, v)
			rep.Lengths = append(rep.Lengths, uint64(len(v)))
			totalLength += len(v)
			continue
		}
		db.DPrintf(db.CACHESRV_ERR, "Key not found %v shard %v", getReq.Key, getReq.Tshard())
		// Key not found, so bail out & fail all gets
		return serr.NewErr(serr.TErrNotfound, fmt.Sprintf("key %s", getReq.Key))
	}
	// Concatenate buffers to speed up blob write
	b := make([]byte, totalLength)
	idx := 0
	for _, buf := range bufs {
		n := copy(b[idx:idx+len(buf)], buf)
		if n != len(buf) {
			db.DFatalf("Didn't copy whole buf: %v != %v", n, len(buf))
		}
		idx += len(buf)
	}
	db.DPrintf(db.CACHESRV, "MultiGet reply total serialized length: %v", len(b))
	rep.Blob.Iov = append(rep.Blob.Iov, b)
	return nil
}

func (cs *CacheSrv) Delete(ctx fs.CtxI, req cacheproto.CacheReq, rep *cacheproto.CacheRep) error {
	if false {
		_, span := cs.tracer.StartRPCSpan(&req, "Delete")
		defer span.End()
	}

	db.DPrintf(db.CACHESRV, "Delete %v", req)

	start := time.Now()

	s, err := cs.lookupShard(req.Tshard())
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
	return serr.NewErr(serr.TErrNotfound, fmt.Sprintf("key %s", req.Key))

}
