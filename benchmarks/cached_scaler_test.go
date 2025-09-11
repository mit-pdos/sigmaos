package benchmarks_test

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/assert"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cacheproto "sigmaos/apps/cache/proto"
	"sigmaos/apps/cossim"
	cossimproto "sigmaos/apps/cossim/proto"
	cossimsrv "sigmaos/apps/cossim/srv"
	epsrv "sigmaos/apps/epcache/srv"
	"sigmaos/benchmarks/loadgen"
	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proc/chunk"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type CachedScalerJobInstance struct {
	jobName          string
	sigmaos          bool
	ncache           int
	cacheMCPU        proc.Tmcpu
	cacheGC          bool
	useEPCache       bool
	useCPP           bool
	nKV              int
	delegatedInit    bool
	topN             int
	ready            chan bool
	warmCachedSrvKID string
	cacheKIDs        map[string]bool
	epcj             *epsrv.EPCacheJob
	msc              *mschedclnt.MSchedClnt
	cm               *cachegrpmgr.CacheMgr
	cc               *cachegrpclnt.CachedSvcClnt
	primaryEPs       []*sp.Tendpoint
	lgs              []*loadgen.LoadGenerator
	putLGs           []*loadgen.LoadGenerator
	doPuts           bool
	keys             []string
	vals             []*cacheproto.CacheString
	dur              []time.Duration
	maxrps           []int
	putDur           []time.Duration
	putMaxrps        []int
	scale            bool
	scaleDelay       time.Duration
	scaling          bool
	lastScaled       time.Time
	warmup           bool
	// CosSim params
	cossimBackend       bool
	cossimNVec          int
	cossimVecDim        int
	cossimMCPU          proc.Tmcpu
	cossimDelegatedInit bool
	cossimNVecToQuery   int
	cossimJ             *cossimsrv.CosSimJob
	*test.RealmTstate
}

func NewCachedScalerJob(ts *test.RealmTstate, jobName string, durs string, maxrpss string, putDurs string, putMaxrpss string, ncache int, cacheMCPU proc.Tmcpu, cacheGC bool, useEPCache bool, nKV int, delegatedInit bool, topN int, scale bool, scaleDelay time.Duration, scalerCachedCPP bool, cossimBackend bool, cossimNVec int, cossimVecDim int, cossimMCPU proc.Tmcpu, cossimDelegatedInit bool, cossimNVecToQuery int) *CachedScalerJobInstance {
	ji := &CachedScalerJobInstance{
		RealmTstate:         ts,
		sigmaos:             true,
		jobName:             jobName,
		ncache:              ncache,
		cacheMCPU:           cacheMCPU,
		cacheGC:             cacheGC,
		useEPCache:          useEPCache,
		useCPP:              scalerCachedCPP,
		cacheKIDs:           make(map[string]bool),
		msc:                 mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET),
		nKV:                 nKV,
		delegatedInit:       delegatedInit,
		topN:                topN,
		scale:               scale,
		scaleDelay:          scaleDelay,
		cossimBackend:       cossimBackend,
		cossimNVec:          cossimNVec,
		cossimVecDim:        cossimVecDim,
		cossimMCPU:          cossimMCPU,
		cossimDelegatedInit: cossimDelegatedInit,
		cossimNVecToQuery:   cossimNVecToQuery,
		ready:               make(chan bool),
	}

	durslice := strings.Split(durs, ",")
	maxrpsslice := strings.Split(maxrpss, ",")
	assert.Equal(ts.Ts.T, len(durslice), len(maxrpsslice), "Non-matching lengths: durs(%v) != maxrpss(%v)", len(durslice), len(maxrpsslice))
	ji.dur = make([]time.Duration, 0, len(durslice))
	ji.maxrps = make([]int, 0, len(durslice))
	for i := range durslice {
		d, err := time.ParseDuration(durslice[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		n, err := strconv.Atoi(maxrpsslice[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		ji.dur = append(ji.dur, d)
		ji.maxrps = append(ji.maxrps, n)
	}

	putDurslice := strings.Split(putDurs, ",")
	putMaxrpsslice := strings.Split(putMaxrpss, ",")
	assert.Equal(ts.Ts.T, len(putDurslice), len(putMaxrpsslice), "Non-matching lengths: putDurs(%v) != putMaxrpss(%v)", len(putDurslice), len(putMaxrpsslice))
	ji.putDur = make([]time.Duration, 0, len(putDurslice))
	ji.putMaxrps = make([]int, 0, len(putDurslice))
	for i := range putDurslice {
		d, err := time.ParseDuration(putDurslice[i])
		assert.Nil(ts.Ts.T, err, "Bad putDuration %v", err)
		n, err := strconv.Atoi(putMaxrpsslice[i])
		assert.Nil(ts.Ts.T, err, "Bad putDuration %v", err)
		ji.putDur = append(ji.putDur, d)
		ji.putMaxrps = append(ji.putMaxrps, n)
	}
	ji.doPuts = !(len(ji.putMaxrps) == 1 && ji.putMaxrps[0] == 0)

	var err error
	if ji.useEPCache {
		ji.epcj, err = epsrv.NewEPCacheJob(ts.SigmaClnt)
		if !assert.Nil(ji.Ts.T, err, "Err new epCacheJob: %v", err) {
			return ji
		}
	}
	ji.cm, err = cachegrpmgr.NewCacheMgrEPCache(ts.SigmaClnt, ji.epcj, ji.jobName, ji.ncache, ji.cacheMCPU, ji.cacheGC)
	if !assert.Nil(ts.Ts.T, err, "Err new cachemgr: %v", err) {
		return ji
	}
	if ji.useEPCache {
		ji.cc = cachegrpclnt.NewCachedSvcClntEPCache(ts.FsLib, ji.epcj.Clnt, ji.jobName)
	} else {
		ji.cc = cachegrpclnt.NewCachedSvcClnt(ts.FsLib, ji.jobName)
	}
	ji.keys, ji.vals, err = ji.writeKVsToCache()
	if !assert.Nil(ji.Ts.T, err, "Err write KVs to cache: %v", err) {
		return ji
	}
	ji.primaryEPs = make([]*sp.Tendpoint, ji.ncache)
	for i := 0; i < ji.ncache; i++ {
		ep, err := ji.cc.GetEndpoint(i)
		if !assert.Nil(ts.Ts.T, err, "Err get primary %v endpoint: %v", i, err) {
			return ji
		}
		ji.primaryEPs[i] = ep
	}
	// Find machines were cached is running
	if _, err := ji.msc.GetMScheds(); !assert.Nil(ts.Ts.T, err, "Err GetMScheds: %v", err) {
		return ji
	}
	nMSched, err := ji.msc.NMSched()
	if !assert.Nil(ts.Ts.T, err, "Err GetNMSched: %v", err) {
		return ji
	}
	time.Sleep(10 * time.Second)
	foundCached := false
	for i := 0; i < 5; i++ {
		runningProcs, err := ji.msc.GetRunningProcs(nMSched)
		if !assert.Nil(ts.Ts.T, err, "Err GetRunningProcs: %v", err) {
			return ji
		}
		for _, p := range runningProcs[ts.GetRealm()] {
			// Record where relevant programs are running
			switch p.GetProgram() {
			case "cached":
				ji.cacheKIDs[p.GetKernelID()] = true
				ji.warmCachedSrvKID = p.GetKernelID()
				db.DPrintf(db.TEST, "cached[%v] running on kernel %v", p.GetPid(), p.GetKernelID())
				foundCached = true
			default:
			}
		}
		if !foundCached {
			db.DPrintf(db.TEST, "Didn't find cached")
		}
		time.Sleep(5 * time.Second)
	}
	if !assert.True(ts.Ts.T, foundCached, "Err didn't find cached srv") {
		return ji
	}

	// Construct CosSim request input
	cossimInputVec := cossim.VectorToSlice(cossim.NewVectors(1, ji.cossimVecDim)[0])
	ranges := []*cossimproto.VecRange{
		&cossimproto.VecRange{
			StartID: 0,
			EndID:   uint64(ji.cossimNVecToQuery - 1),
		},
	}

	// Warm up an msched currently running a cached shard with the cached-scaler
	// bin. No cached-scaler server will be able to run on this machine (the
	// CPU reservation conflicts with that of the cached server), so we can be
	// sure that future servers will try to download the cached-scaler binary
	// from this msched.
	db.DPrintf(db.TEST, "Target kernel to run prewarm with CachedScaler bin: %v", ji.warmCachedSrvKID)
	binBase := "cached-scaler-v"
	if ji.useCPP {
		binBase = "cached-srv-cpp-v"
	}
	err = ji.msc.WarmProcd(ji.warmCachedSrvKID, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), binBase+sp.Version, ts.Ts.ProcEnv().GetSigmaPath(), proc.T_LC)
	if !assert.Nil(ts.Ts.T, err, "Err warming with cached-scaler bin: %v", err) {
		return ji
	}
	var misscnt atomic.Int32
	const SCALE_MISS_BUFFER_PERIOD = 2 * time.Second
	db.DPrintf(db.TEST, "Warmed kid %v with CachedScaler bin", ji.warmCachedSrvKID)
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(ji.lgs, loadgen.NewLoadGenerator(ji.dur[i], ji.maxrps[i], func(r *rand.Rand) (time.Duration, bool) {
			idx := r.Int() % len(ji.keys)
			// Select a key to request
			key := ji.keys[idx]
			v := &cacheproto.CacheString{}
			// Record whether or not a miss is acceptable. A miss is acceptable if
			// scaling is currently happening, or finished recently
			// (within the SCALE_MISS_BUFFER_PERIOD)
			missOK := ji.scaling || time.Since(ji.lastScaled) < SCALE_MISS_BUFFER_PERIOD
			// TODO: have cache do LRU eviction instead of simulating this
			// Make 10% of the key space unavailable during a scaling event to
			// simulate the effect of LRU evictions.
			forceMiss := ji.scaling && (idx < len(ji.keys)/10)
			err := ji.cc.Get(key, v)
			if forceMiss || err != nil {
				// OK to have errors while misses are expected, because server may be
				// registered & picked up by EPCC before it is mounted
				if !missOK && !assert.Nil(ji.Ts.T, err, "Err cc get: %v", err) {
					return 0, false
				}
				db.DPrintf(db.TEST, "Cache miss (key=%v)! force %v genuine %v cnt %v err %v", key, forceMiss, err != nil, misscnt.Add(1), err)
				// Fetch from cossim server
				if ji.cossimBackend {
					_, _, err := ji.cossimJ.Clnt.CosSimLeastLoaded(cossimInputVec, ranges)
					assert.Nil(ji.Ts.T, err, "CosSim req: %v", err)
				} else {
					// Simulate fetching the data with a fixed delay
					time.Sleep(50 * time.Millisecond)
				}
			}
			if !missOK {
				assert.Equal(ji.Ts.T, v.Val, ji.vals[idx].Val, "Unexpected val for key %v: %v", key, v.Val)
			}
			return 0, false
		}))
	}
	if ji.doPuts {
		ji.putLGs = make([]*loadgen.LoadGenerator, 0, len(ji.putDur))
		for i := range ji.putDur {
			ji.putLGs = append(ji.putLGs, loadgen.NewLoadGenerator(ji.putDur[i], ji.putMaxrps[i], func(r *rand.Rand) (time.Duration, bool) {
				idx := r.Int() % len(ji.keys)
				// Select a key to request
				key := ji.keys[idx]
				val := ji.vals[idx]
				missOK := ji.scaling || time.Since(ji.lastScaled) < SCALE_MISS_BUFFER_PERIOD
				if err := ji.cc.Put(key, val); !missOK && !assert.Nil(ji.Ts.T, err, "Err cc put: %v", err) {
					return 0, false
				}
				return 0, false
			}))
		}
	}
	if ji.cossimBackend {
		db.DPrintf(db.TEST, "Start cossimsrv")
		eagerInit := true
		ji.cossimJ, err = cossimsrv.NewCosSimJob(ts.SigmaClnt, ji.epcj, ji.cm, ji.cc, ji.jobName, ji.cossimNVec, ji.cossimVecDim, eagerInit, ji.cossimMCPU, ji.ncache, ji.cossimMCPU, cacheGC, ji.cossimDelegatedInit)
		if !assert.Nil(ts.Ts.T, err, "Error NewCosSimJob: %v", err) {
			return ji
		}
		_, _, err := ji.cossimJ.AddSrv()
		if !assert.Nil(ts.Ts.T, err, "Err Cossim AddSrv: %v", err) {
			return ji
		}
		db.DPrintf(db.TEST, "Done start cossimsrv")
	}
	return ji
}

func (ji *CachedScalerJobInstance) StartCachedScalerJob() {
	db.DPrintf(db.TEST, "Start cached scaler job get dur %v get maxrps %v put dur %v put maxrps %v", ji.dur, ji.maxrps, ji.putDur, ji.putMaxrps)
	ji.warmup = true
	// Warm up load generators
	var wg sync.WaitGroup
	for _, lg := range ji.lgs {
		wg.Add(1)
		go func(lg *loadgen.LoadGenerator, wg *sync.WaitGroup) {
			defer wg.Done()
			lg.Calibrate()
		}(lg, &wg)
	}
	wg.Wait()
	ji.warmup = false
	for _, putLG := range ji.putLGs {
		wg.Add(1)
		go func(putLG *loadgen.LoadGenerator, wg *sync.WaitGroup) {
			defer wg.Done()
			putLG.Calibrate()
		}(putLG, &wg)
	}
	wg.Wait()
	// Start a goroutine to asynchronously scale cached
	go ji.scaleCached()
	wg.Add(1)
	// Start a goroutine to asynchronously run puts
	go func() {
		defer wg.Done()
		for i, putLG := range ji.putLGs {
			db.DPrintf(db.TEST, "Run put load generator rps %v dur %v", ji.maxrps[i], ji.dur[i])
			putLG.Run()
		}
	}()
	for i, lg := range ji.lgs {
		db.DPrintf(db.TEST, "Run get load generator rps %v dur %v", ji.maxrps[i], ji.dur[i])
		lg.Run()
	}
	wg.Wait()
}

func (ji *CachedScalerJobInstance) Wait() {
	for _, lg := range ji.lgs {
		lg.Stats()
	}
	ji.cm.Stop()
}

func (ji *CachedScalerJobInstance) scaleCached() {
	// If not scaling, bail out early
	if !ji.scale {
		return
	}
	time.Sleep(ji.scaleDelay)
	ji.scaling = true
	// TODO: More scaling
	db.DPrintf(db.TEST, "Add scaler server")
	if err := ji.cm.AddScalerServerWithSigmaPath(chunk.ChunkdPath(ji.warmCachedSrvKID), ji.delegatedInit, ji.useCPP); !assert.Nil(ji.Ts.T, err, "Err add scaler server: %v", err) {
		return
	}
	ji.scaling = false
	ji.lastScaled = time.Now()
	db.DPrintf(db.TEST, "Done add scaler server")
}

// Write vector DB to cache srv
func (ji *CachedScalerJobInstance) writeKVsToCache() ([]string, []*cacheproto.CacheString, error) {
	keys := make([]string, ji.nKV)
	vals := make([]*cacheproto.CacheString, ji.nKV)
	for i := range keys {
		key := "key-" + strconv.Itoa(i)
		val := &cacheproto.CacheString{Val: "val-" + strconv.Itoa(i)}
		keys[i] = key
		vals[i] = val
		if err := ji.cc.Put(key, val); err != nil {
			return nil, nil, err
		}
	}
	db.DPrintf(db.TEST, "Done write KVs to cache")
	return keys, vals, nil
}
