package benchmarks_test

import (
	"math"
	"runtime"
	"strings"
	"time"

	"ulambda/benchmarks"
	"ulambda/config"
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/realm"
	"ulambda/test"
)

//
// Functions we use to record and output performance.
//

func runOps(ts *test.Tstate, is []interface{}, op testOp, rs *benchmarks.RawResults) {
	for i := 0; i < len(is); i++ {
		// Pefrormance vars
		nRPC := ts.ReadSeqNo()

		// Ops we are benchmarking
		elapsed := op(ts, time.Now(), is[i])

		// Optional counter
		if i%100 == 0 {
			db.DPrintf("TEST", "i = %v", i)
		}

		// Performance bookeeping
		usecs := float64(elapsed.Microseconds())
		nRPC = ts.ReadSeqNo() - nRPC
		db.DPrintf("TEST2", "Latency: %vus", usecs)
		throughput := float64(1.0) / usecs
		rs.Data[i].Set(throughput, usecs, nRPC)
	}
}

func printResults(rs *benchmarks.RawResults) {
	mean := rs.Mean().Latency
	std := rs.StandardDeviation().Latency
	// Round to 2 decimal points.
	ratio := math.Round((std/mean*100.0)*100.0) / 100.0
	// Get info for the caller.
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		db.DFatalf("Couldn't get caller name")
	}
	fnDetails := runtime.FuncForPC(pc)
	n := fnDetails.Name()
	fnName := n[strings.Index(n, ".")+1:]
	db.DPrintf(db.ALWAYS, "\n\nResults: %v\n=====\nLatency\n-----\nMean: %v (usec) Std: %v (usec)\nStd is %v%% of the mean\n=====\n\n", fnName, mean, std, ratio)
}

// Monitor how many cores have been assigned to a realm.
func monitorCoresAssigned(ts *test.Tstate) *perf.Perf {
	p := perf.MakePerf("TEST")
	go func() {
		cc := config.MakeConfigClnt(fslib.MakeFsLib("test"))
		cfgPath := realm.RealmConfPath(ts.RealmId())
		cfg := &realm.RealmConfig{}
		if err := cc.ReadConfig(cfgPath, cfg); err != nil {
			db.DFatalf("Read config err: %v", err)
		}
		p.TptTick(float64(cfg.NCores))
		for {
			if err := cc.WaitConfigChange(cfgPath); err != nil {
				db.DPrintf(db.ALWAYS, "Error WaitConfigChange: %v", err)
				return
			}
			// Make sure changes don't get put in the same tpt bucket.
			time.Sleep(time.Duration(1000/np.Conf.Perf.CPU_UTIL_SAMPLE_HZ) * time.Millisecond)
			if err := cc.ReadConfig(cfgPath, cfg); err != nil {
				db.DPrintf(db.ALWAYS, "Read config err: %v", err)
				return
			}
			p.TptTick(float64(cfg.NCores))
		}
	}()
	return p
}
