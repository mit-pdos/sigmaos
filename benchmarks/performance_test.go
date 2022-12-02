package benchmarks_test

import (
	"runtime"
	"strings"
	"time"

	"sigmaos/benchmarks"
	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/perf"
	"sigmaos/realm"
	"sigmaos/test"
)

//
// Functions we use to record and output performance.
//

func runOps(ts *test.Tstate, is []interface{}, op testOp, rs *benchmarks.Results) {
	for i := 0; i < len(is); i++ {
		// Ops we are benchmarking
		elapsed, amt := op(ts, is[i])

		// Optional counter
		if i%100 == 0 {
			db.DPrintf("TEST", "i = %v", i)
		}

		db.DPrintf("TEST2", "lat %v amt %v", elapsed, amt)
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
	db.DPrintf(db.ALWAYS, "\n\nResults: %v\n=====%v%v=====\n\n", fnName, lsum, tsum)
}

// Monitor how many cores have been assigned to a realm.
func monitorCoresAssigned(ts *test.Tstate) *perf.Perf {
	p := perf.MakePerfMulti("TEST", ts.RealmId())
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
