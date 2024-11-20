package benchmarks_test

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/benchmarks/loadgen"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type scheddFn func(sc *sigmaclnt.SigmaClnt, pid sp.Tpid, kernelpref []string) time.Duration
type kernelPrefFn func() []string

type MSchedJobInstance struct {
	justCli   bool
	skipstats bool
	dur       []time.Duration
	maxrps    []int
	ready     chan bool
	progname  string
	spawnFn   scheddFn
	kpfn      kernelPrefFn
	kidx      atomic.Int64
	clnts     []*sigmaclnt.SigmaClnt
	lgs       []*loadgen.LoadGenerator
	procCnt   atomic.Int64
	*test.RealmTstate
}

func NewMSchedJob(ts *test.RealmTstate, nclnt int, durs string, maxrpss string, progname string, sfn scheddFn, kernels []string, withKernelPref bool, skipstats bool) *MSchedJobInstance {
	ji := &MSchedJobInstance{}
	ji.ready = make(chan bool)
	ji.progname = progname
	ji.spawnFn = sfn
	ji.skipstats = skipstats
	ji.RealmTstate = ts
	ji.clnts = make([]*sigmaclnt.SigmaClnt, nclnt)
	if withKernelPref {
		ji.kpfn = withKPFnRoundRobin(&ji.kidx, kernels)
	} else {
		ji.kpfn = withNoKPFn()
	}

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
			procCnt := int(ji.procCnt.Add(1))
			pid := sp.Tpid(ji.progname + "-" + strconv.Itoa(procCnt))
			// Run a single request.
			dur := ji.spawnFn(ji.clnts[procCnt%len(ji.clnts)], pid, ji.kpfn())
			return dur, true
		}))
	}
	return ji
}

func withKPFnRoundRobin(idx *atomic.Int64, kids []string) kernelPrefFn {
	return func() []string {
		next := int(idx.Add(1)) % len(kids)
		return kids[next : next+1]
	}
}

func withNoKPFn() kernelPrefFn {
	return func() []string {
		return nil
	}
}

func (ji *MSchedJobInstance) StartMSchedJob() {
	p := newRealmPerf(ji.RealmTstate)
	defer p.Done()

	db.DPrintf(db.ALWAYS, "StartMSchedJob dur %v maxrps %v", ji.dur, ji.maxrps)
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
	db.DPrintf(db.ALWAYS, "Done running MSchedJob")
}

func (ji *MSchedJobInstance) Wait() {
	if !ji.skipstats {
		for _, lg := range ji.lgs {
			lg.Stats()
		}
	}
}
