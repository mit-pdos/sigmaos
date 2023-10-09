package benchmarks_test

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/loadgen"
	"sigmaos/sigmaclnt"
	"sigmaos/test"
)

type scheddFn func(*sigmaclnt.SigmaClnt) time.Duration

type ScheddJobInstance struct {
	justCli bool
	dur     []time.Duration
	maxrps  []int
	ready   chan bool
	fn      scheddFn
	clnts   []*sigmaclnt.SigmaClnt
	lgs     []*loadgen.LoadGenerator
	*test.RealmTstate
}

func NewScheddJob(ts *test.RealmTstate, nclnt int, durs string, maxrpss string, fn scheddFn) *ScheddJobInstance {
	ji := &ScheddJobInstance{}
	ji.ready = make(chan bool)
	ji.fn = fn
	ji.RealmTstate = ts
	ji.clnts = make([]*sigmaclnt.SigmaClnt, nclnt)

	// Make clnts for load test
	for i := range ji.clnts {
		var err error
		ji.clnts[i], err = sigmaclnt.NewSigmaClnt(ts.ProcEnv())
		assert.Nil(ts.Ts.T, err, "Error new sigma clnt: %v", err)
	}

	durslice := strings.Split(durs, ",")
	maxrpsslice := strings.Split(maxrpss, ",")
	assert.Equal(ts.Ts.T, len(durslice), len(maxrpsslice), "Non-matching lengths: %v %v", durs, maxrpss)

	ji.dur = make([]time.Duration, 0, len(durslice))
	ji.maxrps = make([]int, 0, len(durslice))

	for i := range durslice {
		d, err := time.ParseDuration(durslice[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		n, err := strconv.Atoi(maxrpsslice[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		ji.dur = append(ji.dur, d)
		ji.maxrps = append(ji.maxrps, n)
	}

	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(ji.lgs, loadgen.NewLoadGenerator(ji.dur[i], ji.maxrps[i], func(r *rand.Rand) (time.Duration, bool) {
			// Run a single request.
			dur := ji.fn(ji.clnts[i])
			return dur, true
		}))
	}
	return ji
}

func (ji *ScheddJobInstance) StartScheddJob() {
	db.DPrintf(db.ALWAYS, "StartScheddJob dur %v maxrps %v", ji.dur, ji.maxrps)
	var wg sync.WaitGroup
	for _, lg := range ji.lgs {
		wg.Add(1)
		go func(lg *loadgen.LoadGenerator, wg *sync.WaitGroup) {
			defer wg.Done()
			lg.Calibrate()
		}(lg, &wg)
	}
	wg.Wait()
	for i, lg := range ji.lgs {
		db.DPrintf(db.TEST, "Run load generator rps %v dur %v", ji.maxrps[i], ji.dur[i])
		lg.Run()
	}
	db.DPrintf(db.ALWAYS, "Done running ScheddJob")
}

func (ji *ScheddJobInstance) Wait() {
	for _, lg := range ji.lgs {
		lg.Stats()
	}
}
