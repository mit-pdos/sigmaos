package clnt_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	"sigmaos/apps/cache/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/util/coordination/semaphore"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	rd "sigmaos/util/rand"
)

const (
	CACHE_MCPU = 2000
)

type Tstate struct {
	*test.Tstate
	cm    *cachegrpmgr.CacheMgr
	clrks []sp.Tpid
	job   string
	sempn string
	sem   *semaphore.Semaphore
}

func newTstate(t *test.Tstate, nsrv int) *Tstate {
	ts := &Tstate{}
	ts.Tstate = t
	ts.job = rd.String(16)
	ts.Remove(cache.CACHE)
	cm, err := cachegrpmgr.NewCacheMgr(ts.SigmaClnt, ts.job, nsrv, proc.Tmcpu(CACHE_MCPU), true)
	assert.Nil(t.T, err)
	ts.cm = cm
	ts.sempn = cm.SvcDir() + "-cacheclerk-sem"
	ts.sem = semaphore.NewSemaphore(ts.FsLib, ts.sempn)
	err = ts.sem.Init(0)
	assert.Nil(t.T, err)
	return ts
}

func (ts *Tstate) stop() {
	db.DPrintf(db.ALWAYS, "wait for %d clerks to exit\n", len(ts.clrks))
	for _, ck := range ts.clrks {
		opTpt, err := cachegrpclnt.WaitClerk(ts.SigmaClnt, ck)
		assert.Nil(ts.T, err, "StopClerk: %v", err)
		db.DPrintf(db.ALWAYS, "clerk %v %v ops/sec", ck, opTpt)
	}
	ts.cm.Stop()
}

func (ts *Tstate) StartClerk(dur time.Duration, nkeys, keyOffset int, mcpu proc.Tmcpu) {
	pid, err := cachegrpclnt.StartClerk(ts.SigmaClnt, ts.job, nkeys, dur, keyOffset, ts.sempn, mcpu)
	assert.Nil(ts.T, err, "Error StartClerk: %v", err)
	ts.clrks = append(ts.clrks, pid)
}

func TestCompile(t *testing.T) {
}

func TestCacheSingle(t *testing.T) {
	const (
		N    = 10000
		NSRV = 1
	)
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, NSRV)
	cc := cachegrpclnt.NewCachedSvcClnt(ts.FsLib, ts.job)

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		err := cc.Put(key, &proto.CacheString{Val: key})
		assert.Nil(t, err)
	}
	t0 := time.Now()
	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		res := &proto.CacheString{}
		err := cc.Get(key, res)
		s := res.Val
		assert.Nil(t, err)
		assert.Equal(t, key, s)
	}
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "Get %v keys in %v ms (%v us per Get)\n", N, ms, (ms*1000)/N)

	m, err := cc.Dump(0)
	assert.Nil(t, err)
	assert.Equal(t, N, len(m))

	m, err = cc.Dump(0)
	assert.Nil(t, err)
	assert.Equal(t, N, len(m))

	// Delete and get
	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		res := &proto.CacheString{}
		err = cc.Delete(key)
		assert.Nil(t, err)
		err = cc.Get(key, res)
		assert.True(t, cache.IsMiss(err))
	}

	cc.Close()
	ts.Shutdown()
}

func testCacheSharded(t *testing.T, nsrv int) {
	const (
		N = 10
	)
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, nsrv)
	cc := cachegrpclnt.NewCachedSvcClnt(ts.FsLib, ts.job)

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		err := cc.Put(key, &proto.CacheString{Val: key})
		assert.Nil(t, err)
	}

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		res := &proto.CacheString{}
		err := cc.Get(key, res)
		s := res.Val
		assert.Nil(t, err)
		assert.Equal(t, key, s)
	}

	for g := 0; g < nsrv; g++ {
		m, err := cc.Dump(g)
		assert.Nil(t, err)
		assert.True(t, len(m) >= 1)
	}

	// Delete and get
	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		res := &proto.CacheString{}
		err := cc.Delete(key)
		assert.Nil(t, err)
		err = cc.Get(key, res)
		assert.True(t, cache.IsMiss(err))
	}

	cc.Close()
	ts.stop()
	ts.Shutdown()
}

func TestCacheShardedTwo(t *testing.T) {
	testCacheSharded(t, 2)
}

func TestCacheShardedThree(t *testing.T) {
	testCacheSharded(t, 3)
}

func TestCacheConcur(t *testing.T) {
	const (
		N    = 3
		NSRV = 1
	)
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, NSRV)
	v := "hello"
	cc := cachegrpclnt.NewCachedSvcClnt(ts.FsLib, ts.job)
	err := cc.Put("x", &proto.CacheString{Val: v})
	assert.Nil(t, err)

	wg := &sync.WaitGroup{}
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := &proto.CacheString{}
			err = cc.Get("x", res)
			s := res.Val
			assert.Equal(t, v, s)
			db.DPrintf(db.TEST, "Done get")
		}()
	}
	wg.Wait()

	cc.Close()
	ts.stop()
	ts.Shutdown()
}

func TestCacheClerk(t *testing.T) {
	const (
		N     = 2
		NSRV  = 2
		NKEYS = 100
		DUR   = 10 * time.Second
	)

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, NSRV)

	for i := 0; i < N; i++ {
		ts.StartClerk(DUR, NKEYS, i*NKEYS, 0)
	}

	ts.sem.Up()

	ts.stop()
	ts.Shutdown()
}

func TestElasticCache(t *testing.T) {
	const (
		N     = 2
		NSRV  = 1
		NKEYS = 100
		DUR   = 30 * time.Second
	)

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts := newTstate(t1, NSRV)

	for i := 0; i < N; i++ {
		ts.StartClerk(DUR, NKEYS, i*NKEYS, 2*1000)
	}

	ts.sem.Up()

	cc := cachegrpclnt.NewCachedSvcClnt(ts.FsLib, ts.job)

	for i := 0; i < 5; i++ {
		time.Sleep(5 * time.Second)
		sts, err := cc.StatsSrvs()
		assert.Nil(t, err)
		qlen := sts[0].StatsSnapshot.AvgQlen
		db.DPrintf(db.ALWAYS, "Qlen %v %v\n", qlen, sts)
		if qlen > 1.1 && i < 1 {
			db.DPrintf(db.TEST, "Add server")
			ts.cm.AddServer()
		}
	}

	cc.Close()
	ts.stop()
	ts.Shutdown()
}
