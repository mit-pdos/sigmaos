package sigmap_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
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
		if err := cc.PutBytes(key, []byte("val"+strconv.Itoa(i))); err != nil {
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

	_ = keys
}
