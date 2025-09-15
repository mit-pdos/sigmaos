package sigmap_test

import (
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cacheclnt "sigmaos/apps/cache/clnt"
	cacheproto "sigmaos/apps/cache/proto"
	cachesrv "sigmaos/apps/cache/srv"
	epsrv "sigmaos/apps/epcache/srv"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
}

// Write vector DB to cache srv
func writeKVsToCache(cc *cachegrpclnt.CachedSvcClnt, nkv int) ([]string, error) {
	keys := make([]string, nkv)
	for i := range keys {
		key := "key-" + strconv.Itoa(i)
		keys[i] = key
		if err := cc.Put(key, &cacheproto.CacheString{Val: "val-" + strconv.Itoa(i)}); err != nil {
			return nil, err
		}
	}
	db.DPrintf(db.TEST, "Done write KVs to cache")
	return keys, nil
}

func TestCachedDelegatedReshard(t *testing.T) {
	const (
		JOB_NAME          = "scalecache-job"
		ncache            = 1
		cacheMcpu         = 3000
		cacheGC           = true
		useEPCache        = true
		N_KV              = 5000
		N_HOTSHARD_TRIALS = 5
		DELEGATED_INIT    = true
		TOP_N             = cachesrv.GET_ALL_SHARDS
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	var epcj *epsrv.EPCacheJob
	var err error
	if useEPCache {
		epcj, err = epsrv.NewEPCacheJob(mrts.GetRealm(test.REALM1).SigmaClnt)
		if !assert.Nil(t, err, "Err new epCacheJob: %v", err) {
			return
		}
	}

	// Start the cachegrp job
	cm, err := cachegrpmgr.NewCacheMgrEPCache(mrts.GetRealm(test.REALM1).SigmaClnt, epcj, JOB_NAME, ncache, cacheMcpu, cacheGC)
	if !assert.Nil(t, err, "Err new cachemgr: %v", err) {
		return
	}
	cc := cachegrpclnt.NewCachedSvcClnt(mrts.GetRealm(test.REALM1).FsLib, JOB_NAME)
	keys, err := writeKVsToCache(cc, N_KV)
	if !assert.Nil(t, err, "Err write KVs to cache: %v", err) {
		return
	}
	srvID := 0
	ep, err := cc.GetEndpoint(srvID)
	if !assert.Nil(t, err, "Err get primary endpoint: %v", err) {
		return
	}
	// Sleep for a bit, for the list of hot shards to populate
	time.Sleep(2 * cachesrv.SHARD_STAT_SCAN_INTERVAL)
	if err := cm.AddBackupServer(srvID, ep, DELEGATED_INIT, TOP_N); !assert.Nil(t, err, "Err add backup server(%v): %v", srvID, err) {
		return
	}
	var foundEnoughMatches bool
	// May need to retry, as shard hit counts reset periodically
	for i := 0; i < N_HOTSHARD_TRIALS; i++ {
		shardHits := make(map[uint32]uint64)
		for i, key := range keys {
			if _, ok := shardHits[cc.Key2shard(key)]; !ok {
				shardHits[cc.Key2shard(key)] = 0
			}
			shardHits[cc.Key2shard(key)]++
			val := &cacheproto.CacheString{}
			// Try getting from the original server
			if err := cc.Get(key, val); !assert.Nil(t, err, "Err get cachemgr: %v", err) {
				break
			}
			if !assert.Equal(t, val.Val, "val-"+strconv.Itoa(i), "Err vals don't match") {
				break
			}
		}
		hotShards, hitCnts, err := cc.GetHotShards(0, cache.NSHARD)
		if !assert.Nil(t, err, "Err GetHotShards: %v", err) {
			return
		}
		nMatches := 0
		for i := range hotShards {
			// If reported hit count matches expected hit count, increment number of
			// matches
			if hitCnts[i] == shardHits[uint32(hotShards[i])] {
				nMatches++
			}
		}
		// If ~50% of the shard hit counts match, declare success
		if nMatches >= int(math.Round(float64(cache.NSHARD)*0.5)) {
			db.DPrintf(db.TEST, "Found sufficient shard hit matches (%v)", nMatches)
			foundEnoughMatches = true
			break
		} else {
			randSleep := cachesrv.SHARD_STAT_SCAN_INTERVAL + cachesrv.SHARD_STAT_SCAN_INTERVAL/time.Duration(rand.Intn(50)+1)
			db.DPrintf(db.TEST, "Insufficient shard hit matches (%v)... sleep %v", nMatches, randSleep)
			time.Sleep(randSleep)
		}
	}
	if !assert.True(t, foundEnoughMatches, "Didn't find enough shard hit count matches") {
		return
	}
	// Wait a bit for shard counts to reset
	time.Sleep(2 * cachesrv.SHARD_STAT_SCAN_INTERVAL)
	// Check shard counts reset
	_, shardCnts, err := cc.GetHotShards(0, cache.NSHARD)
	if !assert.Equal(t, int(cache.NSHARD), len(shardCnts), "HotShard counts didn't reset") {
		return
	}
	if !assert.Nil(t, err, "Err GetHotShards: %v", err) {
		return
	}
	for _, cnt := range shardCnts {
		if !assert.Equal(t, cnt, uint64(0), "HotShard counts didn't reset") {
			return
		}
	}
	for i, key := range keys {
		val := &cacheproto.CacheString{}
		// Try getting from the backup server
		if err := cc.BackupGet(key, val); !assert.Nil(t, err, "Err backup get cachemgr: %v expected to be in shard: %v", err, cacheclnt.Key2shard(key, cache.NSHARD)) {
			break
		}
		if !assert.Equal(t, val.Val, "val-"+strconv.Itoa(i), "Err vals don't match") {
			break
		}
	}
	db.DPrintf(db.TEST, "Sleep a bit so caches can print stats")
	time.Sleep(5 * time.Second)
	if err := cm.Stop(); !assert.Nil(t, err, "Err stop cachemgr: %v", err) {
		return
	}
}

func TestCachedDelegatedReshardCPPScaler(t *testing.T) {
	const (
		JOB_NAME       = "scalecache-job"
		ncache         = 1
		cacheMcpu      = 1000
		cacheGC        = true
		useEPCache     = true
		N_KV           = 5000
		DELEGATED_INIT = true
		TOP_N          = cachesrv.GET_ALL_SHARDS
		CPP            = true
		SHMEM          = true
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	var epcj *epsrv.EPCacheJob
	var err error
	if useEPCache {
		epcj, err = epsrv.NewEPCacheJob(mrts.GetRealm(test.REALM1).SigmaClnt)
		if !assert.Nil(t, err, "Err new epCacheJob: %v", err) {
			return
		}
	}

	// Start the cachegrp job
	cm, err := cachegrpmgr.NewCacheMgrEPCache(mrts.GetRealm(test.REALM1).SigmaClnt, epcj, JOB_NAME, ncache, cacheMcpu, cacheGC)
	if !assert.Nil(t, err, "Err new cachemgr: %v", err) {
		return
	}
	cc := cachegrpclnt.NewCachedSvcClntEPCache(mrts.GetRealm(test.REALM1).FsLib, epcj.Clnt, JOB_NAME)
	keys, err := writeKVsToCache(cc, N_KV)
	if !assert.Nil(t, err, "Err write KVs to cache: %v", err) {
		return
	}
	// Sleep for a bit, for the list of hot shards to populate
	if err := cm.AddScalerServerWithSigmaPath(sp.NOT_SET, DELEGATED_INIT, CPP, SHMEM); !assert.Nil(t, err, "Err add scaler server(%v): %v", err) {
		return
	}
	time.Sleep(2 * time.Second)
	for i, key := range keys {
		val := &cacheproto.CacheString{}
		// Try getting from the backup server
		if err := cc.Get(key, val); !assert.Nil(t, err, "Err get cachemgr: %v expected to be in shard: %v", err, cacheclnt.Key2shard(key, cache.NSHARD)) {
			break
		}
		if !assert.Equal(t, val.Val, "val-"+strconv.Itoa(i), "Err vals don't match") {
			break
		}
	}
	time.Sleep(5 * time.Second)
	if err := cm.Stop(); !assert.Nil(t, err, "Err stop cachemgr: %v", err) {
		return
	}
}
