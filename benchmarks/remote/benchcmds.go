package remote

import (
	"fmt"
	"strconv"

	"sigmaos/proc"
)

// Constructors for commands used to start benchmarks

func GetInitFSCmd(bcfg *BenchConfig, ccfg *ClusterConfig) string {
	const (
		debugSelectors string = "\"BENCH;TEST;\""
	)
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/fslib -timeout 0 --no-shutdown --etcdIP %s --tag %s "+
		"--run InitFs "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		ccfg.LeaderNodeIP,
		bcfg.Tag,
	)
}

func GetColdStartCmd(bcfg *BenchConfig, ccfg *ClusterConfig) string {
	const (
		debugSelectors string = "\"TEST;BENCH;LOADGEN;SPAWN_LAT;NET_LAT;REALM_GROW_LAT;CACHE_LAT;WALK_LAT;FSETCD_LAT;ATTACH_LAT;CHUNKSRV;CHUNKCLNT;\""
	)
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown --etcdIP %s --tag %s "+
		"--run TestMicroScheddSpawn "+
		"--use_rust_proc "+
		"--schedd_dur 5s "+
		"--schedd_max_rps 8 "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		ccfg.LeaderNodeIP,
		bcfg.Tag,
	)
}

// Construct command string to run MR benchmark.
//
// - mrApp specifies which MR app to run (WC or Grep), as well as the input,
// intermediate, and output data sources/destinations.
//
// - memReq specifies the amount of memory requested by each mapper/reducer.
//
// - If asyncRW is true, use the SigmaOS asynchronous reader/writer
// implementation for mappers and reducers.
//
// - If prewarm is true, warm up the realm by predownloading binaries to the
// SigmaOS nodes.
//
// - If measureTpt is true, set the perf selectors which will monitor
// instantaneous throughput. This is an optional parameter because it adds
// non-insignificant overhead to the MR computation, which unfairly penalizes
// the SigmaOS implementation when comparing to Corral.
func GetMRCmdConstructor(mrApp string, memReq proc.Tmem, asyncRW, prewarmRealm, measureTpt bool) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors        string = "\"TEST;BENCH;MR;\""
			optionalPerfSelectors string = "\"TEST_TPT;BENCH_TPT;\""
		)
		// If measuring throughput, set the perf selectors
		perfSelectors := "\"\""
		if measureTpt {
			perfSelectors = optionalPerfSelectors
		}
		prewarm := ""
		if prewarmRealm {
			prewarm = "--prewarm_realm"
		}
		asyncrw := ""
		if asyncRW {
			asyncrw = "--mr_asyncrw"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown --etcdIP %s --tag %s "+
			"--run AppMR "+
			"%s "+ // prewarm
			"%s "+ // asyncrw
			"--mr_mem_req %s "+
			"--mrapp %s "+
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			perfSelectors,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			prewarm,
			asyncrw,
			strconv.Itoa(int(memReq)),
			mrApp,
		)
	}
}
