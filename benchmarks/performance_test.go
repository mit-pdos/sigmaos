package benchmarks_test

import (
	"runtime"
	"strings"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
	mschedclnt "sigmaos/sched/msched/clnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	k8sutil "sigmaos/util/k8s"
	"sigmaos/util/perf"
)

const (
	CPU_MONITOR_INTERVAL       = 1000 * time.Millisecond
	SCHEDD_STAT_MONITOR_PERIOD = 1000 * time.Millisecond
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

func newRealmPerf(ts *test.RealmTstate) *perf.Perf {
	p, err := perf.NewPerfMulti(ts.ProcEnv(), perf.BENCH, ts.GetRealm().String())
	assert.Nil(ts.Ts.T, err)
	return p
}

func newRealmCostPerf(ts *test.RealmTstate) *perf.Perf {
	p, err := perf.NewPerfMulti(ts.ProcEnv(), perf.COST, ts.GetRealm().String())
	assert.Nil(ts.Ts.T, err)
	return p
}

// Monitor how many cores have been assigned to a realm.
func monitorCPUUtil(ts *test.RealmTstate, p *perf.Perf) {
	sdc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
	go func() {
		for {
			perc, nodePerc, err := sdc.GetCPUUtils(ts.GetRealm())
			if err != nil {
				db.DPrintf(db.ALWAYS, "Error GetCPUUtil: %v", err)
				return
			}
			// Util is returned as a percentage (e.g. 100 = 1 core fully utilized,
			// 200 = 2 cores, etc.). So, convert no # of cores by dividing by 100.
			ncores := perc / 100.0
			// What the nodes are doing in total, in cores. Unlike the realm
			// figure this counts every cgroup on them — including the realm's own
			// ux/s3/chunkd servers, which serve the realm's I/O but sit outside
			// its procd containers — so the two together show how much of a
			// realm's work is invisible to the realm figure. It is a property of
			// the cluster, identical for every realm, so it is logged rather than
			// fed to TptTick, which accumulates per-realm series.
			nodeNCores := nodePerc / 100.0
			// Total CPU utilized by this realm (in cores).
			p.TptTick(ncores)
			// "<realm's cores>, <every cgroup on those nodes>".
			db.DPrintf(db.BENCH, "[%v] Cores utilized: %v, %v", ts.GetRealm(), ncores, nodeNCores)
			time.Sleep(CPU_MONITOR_INTERVAL)
		}
	}()
}

func monitorK8sCPUUtil(ts *test.RealmTstate, p *perf.Perf, app string, realm sp.Trealm) {
	go func() {
		for {
			top := k8sTop()
			util := parseK8sUtil(top, app, realm)
			p.TptTick(util)
			db.DPrintf(db.BENCH, "[%v] Cores utilized: %v", ts.GetRealm(), util)
			time.Sleep(CPU_MONITOR_INTERVAL)
		}
	}()
}

func monitorK8sCPUUtilScraperTS(ts *test.RealmTstate, p *perf.Perf, qosClass string) {
	clnt := k8sutil.NewStatScraperClnt(ts.SigmaClnt)
	scrapers := clnt.GetStatScrapers()
	db.DPrintf(db.BENCH, "Got %v scrapers: %v", len(scrapers), scrapers)
	go func() {
		for {
			sumUtil := float64(0.0)
			for _, s := range scrapers {
				perc, err := clnt.GetGuaranteedPodCPUUtil(s, qosClass)
				if err != nil {
					db.DPrintf(db.ALWAYS, "Error GetCPUUtil: %v", err)
					return
				}
				// Util is returned as a percentage (e.g. 100 = 1 core fully utilized,
				// 200 = 2 cores, etc.). So, convert no # of cores by dividing by 100.
				ncores := perc / 100.0
				sumUtil += ncores
			}
			if sumUtil < 0.0 {
				sumUtil = 0.0
			}
			p.TptTick(sumUtil)
			db.DPrintf(db.BENCH, "[%v] Cores utilized by %v pods: %v", sp.ROOTREALM, qosClass, sumUtil)
			time.Sleep(CPU_MONITOR_INTERVAL)
		}
	}()
}

func monitorK8sCPUUtilScraper(ts *test.Tstate, p *perf.Perf, qosClass string) {
	clnt := k8sutil.NewStatScraperClnt(ts.SigmaClnt)
	scrapers := clnt.GetStatScrapers()
	db.DPrintf(db.BENCH, "Got %v scrapers: %v", len(scrapers), scrapers)
	go func() {
		for {
			sumUtil := float64(0.0)
			for _, s := range scrapers {
				perc, err := clnt.GetGuaranteedPodCPUUtil(s, qosClass)
				if err != nil {
					db.DPrintf(db.ALWAYS, "Error GetCPUUtil: %v", err)
					return
				}
				// Util is returned as a percentage (e.g. 100 = 1 core fully utilized,
				// 200 = 2 cores, etc.). So, convert no # of cores by dividing by 100.
				ncores := perc / 100.0
				sumUtil += ncores
			}
			if sumUtil < 0.0 {
				sumUtil = 0.0
			}
			p.TptTick(sumUtil)
			db.DPrintf(db.BENCH, "[%v] Cores utilized by %v pods: %v", sp.ROOTREALM, qosClass, sumUtil)
			time.Sleep(CPU_MONITOR_INTERVAL)
		}
	}()
}
