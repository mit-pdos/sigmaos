package cacheclnt_test

import (
	"log"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/semclnt"
	"sigmaos/test"
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
	cm, err := cacheclnt.MkCacheMgr(ts.FsLib, ts.ProcClnt, ts.job, n)
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
		status, err := ts.WaitExit(ck)
		assert.Nil(ts.T, err, "StopClerk: %v", err)
		assert.True(ts.T, status.IsStatusOK(), "Exit status: %v", status)
		log.Printf("clerk %v %v %v\n", ck, status, status.Data().(float64))
	}
	ts.cm.Stop()
}

func (ts *Tstate) StartClerk(args []string, ncore proc.Tcore) {
	args = append([]string{ts.job}, args...)
	p := proc.MakeProc("cache-clerk", args)
	p.SetNcore(ncore)
	// SpawnBurst to spread clerks across procds.
	_, errs := ts.SpawnBurst([]*proc.Proc{p})
	assert.True(ts.T, len(errs) == 0)
	err := ts.WaitStart(p.GetPid())
	assert.Nil(ts.T, err, "Error StartClerk: %v", err)

	ts.clrks = append(ts.clrks, p.GetPid())
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

	m, err := cc.Dump(0)
	assert.Nil(t, err)
	assert.Equal(t, N, len(m))

	m, err = cc.Dump(0)
	assert.Nil(t, err)
	assert.Equal(t, N, len(m))

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
		DUR    = "10s"
	)

	ts := mkTstate(t, NSHARD)

	for i := 0; i < N; i++ {
		args := []string{strconv.Itoa(NKEYS), DUR, strconv.Itoa(i * NKEYS), ts.sempn}
		ts.StartClerk(args, 0)
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
		DUR    = "30s"
	)

	ts := mkTstate(t, NSHARD)

	for i := 0; i < N; i++ {
		args := []string{strconv.Itoa(NKEYS), DUR, strconv.Itoa(i * NKEYS), ts.sempn}
		ts.StartClerk(args, 2)
	}

	ts.sem.Up()

	cc, err := cacheclnt.MkCacheClnt(ts.FsLib, ts.job)
	assert.Nil(t, err)

	for i := 0; i < 5; i++ {
		time.Sleep(5 * time.Second)
		sts, err := cc.StatsSrv()
		assert.Nil(t, err)
		qlen := sts[0].AvgQLen
		log.Printf("Qlen %v\n", qlen)
		if qlen > 1.1 && i < 1 {
			ts.cm.AddShard()
		}
	}

	ts.stop()
	ts.Shutdown()
}
