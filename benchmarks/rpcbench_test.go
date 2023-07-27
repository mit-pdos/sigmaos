package benchmarks_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/loadgen"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/protdev"
	"sigmaos/rpcbench"
	"sigmaos/scheddclnt"
	"sigmaos/test"
	"sigmaos/tracing"
)

type rpcbenchFn func(c *rpcbench.Clnt)

type RPCBenchJobInstance struct {
	sigmaos    bool
	justCli    bool
	k8ssrvaddr string
	jobpath    string
	dur        []time.Duration
	maxrps     []int
	ncache     int
	cachetype  string
	ready      chan bool
	fn         rpcbenchFn
	rj         *rpcbench.RPCBenchJob
	lgs        []*loadgen.LoadGenerator
	p          *perf.Perf
	*test.RealmTstate
}

func MakeRPCBenchJob(ts *test.RealmTstate, p *perf.Perf, mcpu proc.Tmcpu, durs string, maxrpss string, fn rpcbenchFn, justCli bool) *RPCBenchJobInstance {
	ji := &RPCBenchJobInstance{}
	ji.jobpath = "name/rpcbench"
	ji.ready = make(chan bool)
	ji.fn = fn
	ji.RealmTstate = ts
	ji.p = p
	ji.justCli = justCli

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

	if !ji.justCli {
		var err error
		ji.rj, err = rpcbench.MakeRPCBenchJob(ts.SigmaClnt, ji.jobpath, mcpu, test.Overlays)
		assert.Nil(ts.Ts.T, err, "Error MakeRPCBenchJob: %v", err)
		sdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt.FsLib)
		procs := sdc.GetRunningProcs()
		progs := make(map[string][]string)
		for sd, ps := range procs {
			progs[sd] = make([]string, 0, len(ps))
			for _, p := range ps {
				progs[sd] = append(progs[sd], p.Program)
			}
		}
		db.DPrintf(db.TEST, "Running procs:%v", progs)
	}

	t := tracing.Init("Bench", proc.GetSigmaJaegerIP())

	clnt := rpcbench.MakeClnt(ts.SigmaClnt, t, ji.jobpath)
	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(ji.lgs, loadgen.MakeLoadGenerator(ji.dur[i], ji.maxrps[i], func(r *rand.Rand) {
			// Run a single request.
			ji.fn(clnt)
		}))
	}
	return ji
}

func (ji *RPCBenchJobInstance) StartRPCBenchJob() {
	db.DPrintf(db.ALWAYS, "StartRPCBenchJob dur %v ncache %v maxrps %v kubernetes (%v,%v)", ji.dur, ji.ncache, ji.maxrps, !ji.sigmaos, ji.k8ssrvaddr)
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
	db.DPrintf(db.ALWAYS, "Done running RPCBenchJob")
}

func (ji *RPCBenchJobInstance) printStats() {
	if ji.sigmaos && !ji.justCli {
		stats := &protdev.SigmaRPCStats{}
		s := ji.jobpath
		err := ji.GetFileJson(s+"/"+protdev.STATS, stats)
		assert.Nil(ji.Ts.T, err, "error get stats %v", err)
		fmt.Printf("= %s: %v\n", s, stats)
	}
}

func (ji *RPCBenchJobInstance) Wait() {
	db.DPrintf(db.TEST, "extra sleep")
	time.Sleep(10 * time.Second)
	if ji.p != nil {
		ji.p.Done()
	}
	db.DPrintf(db.TEST, "Evicting hotel procs")
	if ji.sigmaos && !ji.justCli {
		ji.printStats()
		err := ji.rj.Stop()
		assert.Nil(ji.Ts.T, err, "stop %v", err)
	}
	db.DPrintf(db.TEST, "Done evicting hotel procs")
	for _, lg := range ji.lgs {
		db.DPrintf(db.ALWAYS, "Data:\n%v", lg.StatsDataString())
	}
	for _, lg := range ji.lgs {
		lg.Stats()
	}
}
