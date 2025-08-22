package benchmarks_test

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cacheproto "sigmaos/apps/cache/proto"
	epsrv "sigmaos/apps/epcache/srv"
	"sigmaos/benchmarks/loadgen"
	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sched/msched/proc/chunk"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type CachedBackupJobInstance struct {
	jobName          string
	sigmaos          bool
	ncache           int
	cacheMCPU        proc.Tmcpu
	cacheGC          bool
	useEPCache       bool
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
	keys             []string
	dur              []time.Duration
	maxrps           []int
	scale            bool
	scaleDelay       time.Duration
	*test.RealmTstate
}

func NewCachedBackupJob(ts *test.RealmTstate, jobName string, durs string, maxrpss string, ncache int, cacheMCPU proc.Tmcpu, cacheGC bool, useEPCache bool, nKV int, delegatedInit bool, topN int, scale bool, scaleDelay time.Duration) *CachedBackupJobInstance {
	ji := &CachedBackupJobInstance{
		RealmTstate:   ts,
		sigmaos:       true,
		jobName:       jobName,
		ncache:        ncache,
		cacheMCPU:     cacheMCPU,
		cacheGC:       cacheGC,
		useEPCache:    useEPCache,
		cacheKIDs:     make(map[string]bool),
		msc:           mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET),
		nKV:           nKV,
		delegatedInit: delegatedInit,
		topN:          topN,
		scale:         scale,
		scaleDelay:    scaleDelay,
		ready:         make(chan bool),
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
	ji.cc = cachegrpclnt.NewCachedSvcClnt(ts.FsLib, ji.jobName)
	ji.keys, err = ji.writeKVsToCache()
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
			time.Sleep(5 * time.Second)
		}
	}
	if !assert.True(ts.Ts.T, foundCached, "Err didn't find cached srv") {
		return ji
	}
	// Warm up an msched currently running a cached shard with the cached-backup
	// bin. No cached-backup server will be able to run on this machine (the
	// CPU reservation conflicts with that of the cached server), so we can be
	// sure that future servers will try to download the cached-backup binary
	// from this msched.
	db.DPrintf(db.TEST, "Target kernel to run prewarm with CachedBackup bin: %v", ji.warmCachedSrvKID)
	err = ji.msc.WarmProcd(ji.warmCachedSrvKID, ts.Ts.ProcEnv().GetPID(), ts.GetRealm(), "cached-backup-v"+sp.Version, ts.Ts.ProcEnv().GetSigmaPath(), proc.T_LC)
	if !assert.Nil(ts.Ts.T, err, "Err warming with cached-backup bin: %v", err) {
		return ji
	}
	db.DPrintf(db.TEST, "Warmed kid %v with CachedBackup bin", ji.warmCachedSrvKID)
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(ji.lgs, loadgen.NewLoadGenerator(ji.dur[i], ji.maxrps[i], func(r *rand.Rand) (time.Duration, bool) {
			idx := r.Int() % len(ji.keys)
			// Select a key to request
			key := ji.keys[idx]
			v := &cacheproto.CacheString{}
			if err := ji.cc.Get(key, v); !assert.Nil(ji.Ts.T, err, "Err cc get: %v", err) {
				return 0, false
			}
			assert.Equal(ji.Ts.T, v.Val, "val-"+strconv.Itoa(i), "Unexpected val for key %v: %v", key, v.Val)
			// TODO: on miss, try from DB
			// TODO: on MOVE, wait & then retry
			return 0, false
		}))
	}
	return ji
}

func (ji *CachedBackupJobInstance) StartCachedBackupJob() {
	db.DPrintf(db.TEST, "Start cached backup job dur %v maxrps %v", ji.dur, ji.maxrps)
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
	// Start a goroutine to asynchronously scale cached
	go ji.scaleCached()
	for i, lg := range ji.lgs {
		db.DPrintf(db.TEST, "Run load generator rps %v dur %v", ji.maxrps[i], ji.dur[i])
		lg.Run()
	}
}

func (ji *CachedBackupJobInstance) Wait() {
	ji.cm.Stop()
}

func (ji *CachedBackupJobInstance) scaleCached() {
	// If not scaling, bail out early
	if !ji.scale {
		return
	}
	time.Sleep(ji.scaleDelay)
	// TODO: More scaling
	db.DPrintf(db.TEST, "Add backup server")
	srvID := 0
	if err := ji.cm.AddBackupServerWithSigmaPath(chunk.ChunkdPath(ji.warmCachedSrvKID), srvID, ji.primaryEPs[srvID], ji.delegatedInit, ji.topN); !assert.Nil(ji.Ts.T, err, "Err add backup server(%v): %v", srvID, err) {
		return
	}
	db.DPrintf(db.TEST, "Done add backup server")
}

// Write vector DB to cache srv
func (ji *CachedBackupJobInstance) writeKVsToCache() ([]string, error) {
	keys := make([]string, ji.nKV)
	for i := range keys {
		key := "key-" + strconv.Itoa(i)
		keys[i] = key
		if err := ji.cc.Put(key, &cacheproto.CacheString{Val: "val-" + strconv.Itoa(i)}); err != nil {
			return nil, err
		}
	}
	db.DPrintf(db.TEST, "Done write KVs to cache")
	return keys, nil
}
