package cacheclnt_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/cache/proto"
	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/semclnt"
	"sigmaos/test"
)

const (
	CACHE_NCORE = 2
)

type Tstate struct {
	*test.Tstate
	cm    *cacheclnt.CacheMgr
	clrks []proc.Tpid
	job   string
	sempn string
	sem   *semclnt.SemClnt
}

func mkTstate(t *testing.T, n int) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.job = rd.String(16)
	cm, err := cacheclnt.MkCacheMgr(ts.SigmaClnt, ts.job, n, proc.Tcore(CACHE_NCORE), test.Overlays)
	assert.Nil(t, err)
	ts.cm = cm
	ts.sempn = cm.SvcDir() + "-cacheclerk-sem"
	ts.sem = semclnt.MakeSemClnt(ts.FsLib, ts.sempn)
	err = ts.sem.Init(0)
	assert.Nil(t, err)
	return ts
}

func (ts *Tstate) stop() {
	db.DPrintf(db.ALWAYS, "wait for %d clerks to exit\n", len(ts.clrks))
	for _, ck := range ts.clrks {
		opTpt, err := cacheclnt.WaitClerk(ts.SigmaClnt, ck)
		assert.Nil(ts.T, err, "StopClerk: %v", err)
		db.DPrintf(db.ALWAYS, "clerk %v %v ops/sec", ck, opTpt)
	}
	ts.cm.Stop()
}

func (ts *Tstate) StartClerk(dur time.Duration, nkeys, keyOffset int, ncore proc.Tcore) {
	pid, err := cacheclnt.StartClerk(ts.SigmaClnt, ts.job, nkeys, dur, keyOffset, ts.sempn, ncore)
	assert.Nil(ts.T, err, "Error StartClerk: %v", err)
	ts.clrks = append(ts.clrks, pid)
}

func TestCacheSingle(t *testing.T) {
	const (
		N      = 1
		NSHARD = 1
	)

	ts := mkTstate(t, NSHARD)
	cc, err := cacheclnt.MkCacheClnt(ts.FsLib, ts.job)
	assert.Nil(t, err)

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		err = cc.Put(key, &proto.CacheString{Val: key})
		assert.Nil(t, err)
	}
	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		res := &proto.CacheString{}
		err = cc.Get(key, res)
		s := res.Val
		assert.Nil(t, err)
		assert.Equal(t, key, s)
	}
	
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
		assert.True(t, cc.IsMiss(err))
	}

	ts.Shutdown()
}

func testCacheSharded(t *testing.T, nshard int) {
	const (
		N = 10
	)
	ts := mkTstate(t, nshard)
	cc, err := cacheclnt.MkCacheClnt(ts.FsLib, ts.job)
	assert.Nil(t, err)

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		err = cc.Put(key, &proto.CacheString{Val: key})
		assert.Nil(t, err)
	}

	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		res := &proto.CacheString{}
		err = cc.Get(key, res)
		s := res.Val
		assert.Nil(t, err)
		assert.Equal(t, key, s)
	}

	for g := 0; g < nshard; g++ {
		m, err := cc.Dump(g)
		assert.Nil(t, err)
		assert.True(t, len(m) >= 1)
	}

	// Delete and get
	for k := 0; k < N; k++ {
		key := strconv.Itoa(k)
		res := &proto.CacheString{}
		err = cc.Delete(key)
		assert.Nil(t, err)
		err = cc.Get(key, res)
		assert.True(t, cc.IsMiss(err))
	}

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
		N      = 3
		NSHARD = 1
	)
	ts := mkTstate(t, NSHARD)
	v := "hello"
	cc, err := cacheclnt.MkCacheClnt(ts.FsLib, ts.job)
	assert.Nil(t, err)
	err = cc.Put("x", &proto.CacheString{Val: v})
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

	ts.stop()
	ts.Shutdown()
}

func TestCacheClerk(t *testing.T) {
	const (
		N      = 2
		NSHARD = 2
		NKEYS  = 100
		DUR    = 10 * time.Second
	)

	ts := mkTstate(t, NSHARD)

	for i := 0; i < N; i++ {
		ts.StartClerk(DUR, NKEYS, i*NKEYS, 0)
	}

	ts.sem.Up()

	ts.stop()
	ts.Shutdown()
}

func TestElasticCache(t *testing.T) {
	const (
		N      = 2
		NSHARD = 1
		NKEYS  = 100
		DUR    = 30 * time.Second
	)

	ts := mkTstate(t, NSHARD)

	for i := 0; i < N; i++ {
		ts.StartClerk(DUR, NKEYS, i*NKEYS, 2)
	}

	ts.sem.Up()

	cc, err := cacheclnt.MkCacheClnt(ts.FsLib, ts.job)
	assert.Nil(t, err)

	for i := 0; i < 5; i++ {
		time.Sleep(5 * time.Second)
		sts, err := cc.StatsSrv()
		assert.Nil(t, err)
		qlen := sts[0].SigmapStat.AvgQlen
		db.DPrintf(db.ALWAYS, "Qlen %v %v\n", qlen, sts)
		if qlen > 1.1 && i < 1 {
			ts.cm.AddShard()
		}
	}

	ts.stop()
	ts.Shutdown()
}
