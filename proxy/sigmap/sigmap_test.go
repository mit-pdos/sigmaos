package sigmap_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cacheproto "sigmaos/apps/cache/proto"
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
		JOB_NAME  = "scalecache-job"
		ncache    = 1
		cacheMcpu = 4000
		cacheGC   = true
		N_KV      = 5000
		//		DELEGATED_INIT = false
		DELEGATED_INIT = true
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	// Start the cachegrp job
	cm, err := cachegrpmgr.NewCacheMgr(mrts.GetRealm(test.REALM1).SigmaClnt, JOB_NAME, ncache, cacheMcpu, cacheGC)
	if !assert.Nil(t, err, "Err new cachemgr: %v", err) {
		db.DPrintf(db.COSSIMSRV_ERR, "Err newCacheMgr: %v", err)
		return
	}
	cc := cachegrpclnt.NewCachedSvcClnt(mrts.GetRealm(test.REALM1).FsLib, JOB_NAME)
	keys, err := writeKVsToCache(cc, N_KV)
	if !assert.Nil(t, err, "Err write KVs to cache: %v", err) {
		return
	}
	srvID := 0
	if err := cm.AddBackupServer(srvID, DELEGATED_INIT); !assert.Nil(t, err, "Err add backup server(%v): %v", srvID, err) {
		return
	}
	for i, key := range keys {
		val := &cacheproto.CacheString{}
		// Try getting from the original server
		if err := cc.Get(key, val); !assert.Nil(t, err, "Err get cachemgr: %v", err) {
			break
		}
		// Try getting from the backup server
		if err := cc.BackupGet(key, val); !assert.Nil(t, err, "Err backup get cachemgr: %v", err) {
			break
		}
		if !assert.Equal(t, val.Val, "val-"+strconv.Itoa(i), "Err vals don't match") {
			break
		}
	}
	if err := cm.Stop(); !assert.Nil(t, err, "Err stop cachemgr: %v", err) {
		return
	}
}
