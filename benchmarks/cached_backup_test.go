package benchmarks_test

import (
	"strconv"

	"github.com/stretchr/testify/assert"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cacheproto "sigmaos/apps/cache/proto"
	epsrv "sigmaos/apps/epcache/srv"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type CachedBackupJobInstance struct {
	jobName       string
	sigmaos       bool
	ncache        int
	cacheMCPU     proc.Tmcpu
	cacheGC       bool
	useEPCache    bool
	nKV           int
	delegatedInit bool
	topN          int
	ready         chan bool
	epcj          *epsrv.EPCacheJob
	cm            *cachegrpmgr.CacheMgr
	cc            *cachegrpclnt.CachedSvcClnt
	primaryEPs    []*sp.Tendpoint
	*test.RealmTstate
}

func NewCachedBackupJob(ts *test.RealmTstate, jobName string, ncache int, cacheMCPU proc.Tmcpu, cacheGC bool, useEPCache bool, nKV int, delegatedInit bool, topN int) *CachedBackupJobInstance {
	ji := &CachedBackupJobInstance{
		RealmTstate:   ts,
		sigmaos:       true,
		jobName:       jobName,
		ncache:        ncache,
		cacheMCPU:     cacheMCPU,
		cacheGC:       cacheGC,
		useEPCache:    useEPCache,
		nKV:           nKV,
		delegatedInit: delegatedInit,
		topN:          topN,
		ready:         make(chan bool),
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
	keys, err := ji.writeKVsToCache()
	if !assert.Nil(ji.Ts.T, err, "Err write KVs to cache: %v", err) {
		return ji
	}
	_ = keys
	// TODO: use keys to generate load
	ji.primaryEPs = make([]*sp.Tendpoint, ji.ncache)
	for i := 0; i < ji.ncache; i++ {
		ep, err := ji.cc.GetEndpoint(i)
		if !assert.Nil(ts.Ts.T, err, "Err get primary %v endpoint: %v", i, err) {
			return ji
		}
		ji.primaryEPs[i] = ep
	}
	return ji
}

func (ji *CachedBackupJobInstance) StartCachedBackupJob() {
	db.DPrintf(db.TEST, "Add backup server")
	// TODO: loadgen
	// TODO: more than one primary server
	srvID := 0
	if err := ji.cm.AddBackupServer(srvID, ji.primaryEPs[srvID], ji.delegatedInit, ji.topN); !assert.Nil(ji.Ts.T, err, "Err add backup server(%v): %v", srvID, err) {
		return
	}
	db.DPrintf(db.TEST, "Done add backup server")
}

func (ji *CachedBackupJobInstance) Wait() {
	ji.cm.Stop()
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
