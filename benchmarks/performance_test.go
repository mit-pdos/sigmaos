package benchmarks_test

import (
	"runtime"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/proc"
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
	lsum, tsum := rs.Summary()
	db.DPrintf(db.ALWAYS, "\n\nResults: %v\n=====%v%v\n=====\n\n", fnName, lsum, tsum)
}

// Monitor how many cores have been assigned to a realm.
func monitorCoresAssigned(ts *test.RealmTstate, nClusterCores proc.Tcore) *perf.Perf {
	p, err := perf.MakePerfMulti(perf.BENCH, ts.GetRealm().String())
	assert.Nil(ts.T, err)

	go func() {
		sdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt, ts.GetRealm())
		for {
			// TODO: get CPU usage percentage from docker.
			_ = sdc
			percent := 0.0
			// Total CPU utilized by this realm (in cores).
			p.TptTick(float64(percent) * float64(nClusterCores))
			time.Sleep(1 * time.Second)
		}
	}()

	return p
}
