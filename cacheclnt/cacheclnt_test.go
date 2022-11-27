package cacheclnt_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/groupmgr"
	rd "sigmaos/rand"
	"sigmaos/test"
)

type Tstate struct {
	*test.Tstate
	cm      *cacheclnt.CacheMgr
	grpmgrs []*groupmgr.GroupMgr
}

func mkTstate(t *testing.T, n int) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.cm = cacheclnt.MkCacheMgr(ts.FsLib, ts.ProcClnt, rd.String(8), n)
	ts.cm.StartCache()
	return ts
}

func TestCacheSingle(t *testing.T) {
	const (
		N      = 1
		NSHARD = 1
	)

	ts := mkTstate(t, NSHARD)
	cc, err := cacheclnt.MkCacheClnt(ts.FsLib, NSHARD)
	assert.Nil(t, err)

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		err = cc.Set(key, key)
		assert.Nil(t, err)
	}
	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		s := string("")
		err = cc.Get(key, &s)
		assert.Nil(t, err)
		assert.Equal(t, key, s)
	}
	ts.cm.StopCache()
	ts.Shutdown()
}

func testCacheSharded(t *testing.T, nshard int) {
	const (
		N = 10
	)
	ts := mkTstate(t, nshard)
	cc, err := cacheclnt.MkCacheClnt(ts.FsLib, nshard)
	assert.Nil(t, err)

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		err = cc.Set(key, key)
		assert.Nil(t, err)
	}

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		s := string("")
		err = cc.Get(key, &s)
		assert.Nil(t, err)
		assert.Equal(t, key, s)
	}

	for g := 0; g < nshard; g++ {
		m, err := cc.Dump(g)
		assert.Nil(t, err)
		assert.True(t, len(m) >= 1)
	}

	ts.cm.StopCache()
	ts.Shutdown()
}

func TestCacheShardedTwo(t *testing.T) {
	testCacheSharded(t, 2)
}

func TestCacheShardedFive(t *testing.T) {
	testCacheSharded(t, 5)
}

func TestCacheConcur(t *testing.T) {
	const (
		N      = 3
		NSHARD = 1
	)
	ts := mkTstate(t, NSHARD)
	v := "hello"
	cc, err := cacheclnt.MkCacheClnt(ts.FsLib, NSHARD)
	assert.Nil(t, err)
	err = cc.Set("x", v)
	assert.Nil(t, err)

	wg := &sync.WaitGroup{}
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := string("")
			err = cc.Get("x", &s)
			assert.Equal(t, v, s)
			db.DPrintf("TEST", "Done get")
		}()
	}
	wg.Wait()

	ts.cm.StopCache()
	ts.Shutdown()
}
