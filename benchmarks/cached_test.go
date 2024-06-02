package benchmarks_test

import (
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/cachedsvc"
	"sigmaos/cachedsvcclnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/semclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type CachedJobInstance struct {
	dur       time.Duration
	ncache    int
	job       string
	ckmcpu    proc.Tmcpu
	cachemcpu proc.Tmcpu
	nclerks   int
	nkeys     int
	ready     chan bool
	clerks    []sp.Tpid
	cm        *cachedsvc.CacheMgr
	sempn     string
	sem       *semclnt.SemClnt
	*test.RealmTstate
}

func NewCachedJob(ts *test.RealmTstate, nkeys, ncache, nclerks int, dur time.Duration, ckmcpu, cachemcpu proc.Tmcpu) *CachedJobInstance {
	ji := &CachedJobInstance{}
	ji.dur = dur
	ji.ncache = ncache
	ji.dur = dur
	ji.job = rand.String(8)
	ji.ckmcpu = ckmcpu
	ji.nkeys = nkeys
	ji.cachemcpu = cachemcpu
	ji.ready = make(chan bool)
	ji.nclerks = nclerks
	ji.clerks = make([]sp.Tpid, 0, nclerks)
	ji.RealmTstate = ts
	return ji
}

func (ji *CachedJobInstance) RunCachedJob() {
	cm, err := cachedsvc.NewCacheMgr(ji.SigmaClnt, ji.job, ji.ncache, ji.cachemcpu, CACHE_GC)
	assert.Nil(ji.Ts.T, err, "Error NewCacheMgr: %v", err)
	ji.cm = cm
	ji.sempn = ji.cm.SvcDir() + "-cacheclerk-sem"
	ji.sem = semclnt.NewSemClnt(ji.FsLib, ji.sempn)
	err = ji.sem.Init(0)
	assert.Nil(ji.Ts.T, err, "Err sem init %v", err)

	// Start clerks
	for i := 0; i < ji.nclerks; i++ {
		ck, err := cachedsvcclnt.StartClerk(ji.SigmaClnt, ji.job, ji.nkeys, ji.dur, i*ji.nkeys, ji.sempn, ji.ckmcpu)
		assert.Nil(ji.Ts.T, err, "Err StartClerk: %v", err)
		ji.clerks = append(ji.clerks, ck)
	}
	ji.sem.Up()
	// Stop clerks
	aggTpt := float64(0)
	for _, ck := range ji.clerks {
		tpt, err := cachedsvcclnt.WaitClerk(ji.SigmaClnt, ck)
		db.DPrintf(db.ALWAYS, "Clerk throughput: %v ops/sec", tpt)
		assert.Nil(ji.Ts.T, err, "Err waitclerk %v", err)
		aggTpt += tpt
	}
	db.DPrintf(db.ALWAYS, "Aggregate throughput: %v (ops/sec)", aggTpt)
	ji.cm.Stop()
}
