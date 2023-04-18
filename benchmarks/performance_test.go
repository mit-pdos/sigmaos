package benchmarks_test

import (
	"runtime"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/scheddclnt"
	"sigmaos/test"
)

//
// Functions we use to record and output performance.
//

func runOps(ts *test.RealmTstate, is []interface{}, op testOp, rs *benchmarks.Results) {
	for i := 0; i < len(is); i++ {
		// Ops we are benchmarking
		elapsed, amt := op(ts, is[i])

		// Optional counter
		if i%100 == 0 {
			db.DPrintf(db.TEST, "i = %v", i)
		}

		db.DPrintf(db.BENCH, "lat %v amt %v", elapsed, amt)
		rs.Append(elapsed, amt)
	}
}

func printResultSummary(rs *benchmarks.Results) {
	// Get info for the caller.
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		db.DFatalf("Couldn't get caller name")
	}
	fnDetails := runtime.FuncForPC(pc)
	n := fnDetails.Name()
	fnName := n[strings.Index(n, ".")+1:]
	db.DPrintf(db.TEST, "Start print results")
	lsum, tsum := rs.Summary()
	db.DPrintf(db.ALWAYS, "\n\nResults: %v\n=====%v%v\n=====\n\n", fnName, lsum, tsum)
	db.DPrintf(db.TEST, "Done print results")
}

func makeRealmPerf(ts *test.RealmTstate) *perf.Perf {
	p, err := perf.MakePerfMulti(perf.BENCH, ts.GetRealm().String())
	assert.Nil(ts.T, err)
	return p
}

// Monitor how many cores have been assigned to a realm.
func monitorCPUUtil(ts *test.RealmTstate, p *perf.Perf) {
	go func() {
		sdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt, ts.GetRealm())
		for {
			perc, err := sdc.GetCPUUtil()
			if err != nil {
				db.DPrintf(db.ALWAYS, "Error GetCPUUtil: %v", err)
				return
			}
			// Util is returned as a percentage (e.g. 100 = 1 core fully utilized,
			// 200 = 2 cores, etc.). So, convert no # of cores by dividing by 100.
			ncores := perc / 100.0
			// Total CPU utilized by this realm (in cores).
			p.TptTick(ncores)
			db.DPrintf(db.BENCH, "[%v] Cores utilized: %v", ts.GetRealm(), ncores)
			time.Sleep(1 * time.Second)
		}
	}()
}
