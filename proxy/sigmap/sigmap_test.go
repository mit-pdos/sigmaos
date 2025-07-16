package sigmap_test

import (
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

func TestCachedDelegatedReshard(t *testing.T) {
	const (
		JOB_NAME  = "scalecache-job"
		ncache    = 1
		cacheMcpu = 4000
		cacheGC   = true
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
	_ = cm
	_ = cc
}
