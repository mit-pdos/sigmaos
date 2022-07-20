package benchmarks_test

import (
	"math"
	"runtime"
	"strings"
	"time"

	"ulambda/benchmarks"
	db "ulambda/debug"
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
