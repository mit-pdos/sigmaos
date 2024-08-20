package remote

import (
	"fmt"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
)

// Constructors for commands used to start benchmarks

func GetInitFSCmd(bcfg *BenchConfig, ccfg *ClusterConfig) string {
	const (
		debugSelectors string = "\"BENCH;TEST;\""
	)
	netproxy := ""
	if bcfg.NoNetproxy {
		netproxy = "--nonetproxy"
	}
	overlays := ""
	if bcfg.Overlays {
		overlays = "--overlays"
	}
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/fslib -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
		"--run InitFs "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		netproxy,
		overlays,
		ccfg.LeaderNodeIP,
		bcfg.Tag,
	)
}

func GetStartCmdConstructor(rps int, dur time.Duration, prewarmRealm bool) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors string = "\"TEST;BENCH;LOADGEN;\""
		)
		prewarm := ""
		if prewarmRealm {
			prewarm = "--prewarm_realm"
		}
		netproxy := ""
		if bcfg.NoNetproxy {
			netproxy = "--nonetproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run TestMicroScheddSpawn "+
			"--use_rust_proc "+
			"--schedd_dur %s "+
			"--schedd_max_rps %s "+
			"%s "+ // prewarmRealm
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			netproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			dur.String(),
			strconv.Itoa(rps),
			prewarm,
		)
	}
}

// Construct command string to run BE imgresize multiplexing benchmark
func GetBEImgresizeMultiplexingCmd(bcfg *BenchConfig, ccfg *ClusterConfig) string {
	const (
		debugSelectors string = "\"TEST;BENCH;\""
	)
	netproxy := ""
	if bcfg.NoNetproxy {
		netproxy = "--nonetproxy"
	}
	overlays := ""
	if bcfg.Overlays {
		overlays = "--overlays"
	}
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
		"--run TestRealmBalanceImgResizeImgResize "+
		"--sleep 60s "+
		"--n_imgresize 40 "+
		"--imgresize_nround 300 "+
		"--n_imgresize_per 25 "+
		"--imgresize_path name/ux/~local/8.jpg "+
		"--imgresize_mcpu 0 "+
		"--imgresize_mem 1500 "+
		"--nrealm 4 "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		netproxy,
		overlays,
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
			optionalPerfSelectors string = "\"TEST_TPT;BENCH_TPT;THUMBNAIL_TPT;\""
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
		netproxy := ""
		if bcfg.NoNetproxy {
			netproxy = "--nonetproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run AppMR "+
			"%s "+ // prewarm
			"%s "+ // asyncrw
			"--mr_mem_req %s "+
			"--mrapp %s "+
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			perfSelectors,
			netproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			prewarm,
			asyncrw,
			strconv.Itoa(int(memReq)),
			mrApp,
		)
	}
}

// Construct command string to run hotel benchmark's load-generating client
//
// - numClients specifies the total number of client machines which will make
// requests to the hotel application
//
// - rps specifies the number of requests-per-second this client should execute
// in each phase of the benchmark.
//
// - dur specifies the duration for which each rps period should last.
//
// - cacheType specifies the type of cache service that hotel should use (e.g.,
// cached vs kvd vs memcached).
//
// - If scaleCache is true, the cache autoscales.
//
// - clientDelay specifies the delay for which the client should wait before
// starting to send requests.
func GetHotelClientCmdConstructor(leader bool, numClients int, rps []int, dur []time.Duration, cacheType string, scaleCache bool, clientDelay time.Duration) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors string = "\"TEST;THROUGHPUT;CPU_UTIL;\""
			perfSelectors  string = "\"\""
		)
		sys := ""
		if bcfg.K8s {
			sys = "K8s"
		} else {
			sys = "Sigmaos"
		}
		testName := ""
		if leader {
			testName = fmt.Sprintf("Hotel%sSearch", sys)
		} else {
			testName = fmt.Sprintf("Hotel%sJustCliSearch", sys)
		}
		autoscaleCache := ""
		if scaleCache {
			autoscaleCache = "--hotel_cache_autoscale"
		}
		netproxy := ""
		if bcfg.NoNetproxy {
			netproxy = "--nonetproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		k8sFrontendAddr := ""
		if bcfg.K8s {
			addr, err := getK8sHotelFrontendAddr(bcfg, ccfg.lcfg)
			if err != nil {
				db.DFatalf("Get k8s hotel frontend addr:%v", err)
			}
			k8sFrontendAddr = fmt.Sprintf("--k8saddr %s", addr)
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"aws s3 rm --profile sigmaos --recursive s3://9ps3/hotelperf/k8s > /dev/null; "+
			"ulimit -n 100000; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run %s "+
			"--nclnt %s "+
			"--hotel_ncache 3 "+
			"--hotel_cache_mcpu 200 "+
			"--cache_type %s "+
			"%s "+ // scaleCache
			"%s "+ // k8sFrontendAddr
			"--hotel_dur %s "+
			"--hotel_max_rps %s "+
			"--sleep %s "+
			"--prewarm_realm "+
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			perfSelectors,
			netproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			testName,
			strconv.Itoa(numClients),
			cacheType,
			autoscaleCache,
			k8sFrontendAddr,
			dursToString(dur),
			rpsToString(rps),
			clientDelay.String(),
		)
	}
}

// Construct command string to run socialnet benchmark's load-generating client
//
// - numClients specifies the total number of client machines which will make
// requests to the socialnet application
//
// - rps specifies the number of requests-per-second this client should execute
// in each phase of the benchmark.
//
// - dur specifies the duration for which each rps period should last.
func GetSocialnetClientCmdConstructor(leader bool, numClients int, rps []int, dur []time.Duration) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors string = "\"TEST;BENCH;LOADGEN;\""
			perfSelectors  string = "\"\""
		)
		sys := ""
		if bcfg.K8s {
			sys = "K8s"
		} else {
			sys = "Sigmaos"
		}
		testName := ""
		if leader {
			testName = fmt.Sprintf("SocialNet%s", sys)
		} else {
			testName = fmt.Sprintf("SocialNetJustCli%s", sys)
		}
		netproxy := ""
		if bcfg.NoNetproxy {
			netproxy = "--nonetproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"aws s3 rm --profile sigmaos --recursive s3://9ps3/hotelperf/k8s > /dev/null; "+
			"ulimit -n 100000; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run %s "+
			"--nclnt %s "+
			"--sn_read_only "+
			"--sn_dur %s "+
			"--sn_max_rps %s "+
			"--mongourl %s "+
			"--prewarm_realm "+
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			perfSelectors,
			netproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			testName,
			strconv.Itoa(numClients),
			dursToString(dur),
			rpsToString(rps),
			ccfg.LeaderNodeIP+":4407",
		)
	}
}

// Construct command string to run hotel benchmark's load-generating client
//
// - numClients specifies the total number of client machines which will make
// requests to the hotel application
//
// - rps specifies the number of requests-per-second this client should execute
// in each phase of the benchmark.
//
// - dur specifies the duration for which each rps period should last.
//
// - cacheType specifies the type of cache service that hotel should use (e.g.,
// cached vs kvd vs memcached).
//
// - If scaleCache is true, the cache autoscales.
//
// - clientDelay specifies the delay for which the client should wait before
// starting to send requests.
func GetLCBEHotelImgresizeMultiplexingCmdConstructor(numClients int, rps []int, dur []time.Duration, cacheType string, scaleCache bool, clientDelay time.Duration) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors string = "\"TEST;BENCH;CPU_UTIL;IMGD;GROUPMGR;\""
			perfSelectors  string = "\"\""
		)
		autoscaleCache := ""
		if scaleCache {
			autoscaleCache = "--hotel_cache_autoscale"
		}
		netproxy := ""
		if bcfg.NoNetproxy {
			netproxy = "--nonetproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"ulimit -n 100000; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run RealmBalanceHotelImgResize "+
			"--nclnt %s "+
			"--hotel_ncache 3 "+
			"--hotel_cache_mcpu 200 "+
			"--cache_type %s "+
			"%s "+ // scaleCache
			"--hotel_dur %s "+
			"--hotel_max_rps %s "+
			"--sleep %s "+
			"--n_imgresize 590 "+
			"--n_imgresize_per 1 "+
			"--imgresize_path name/ux/~local/8.jpg "+
			"--imgresize_mcpu 0 "+
			"--imgresize_mem 1500 "+
			"--imgresize_nround 500 "+
			"--prewarm_realm "+
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			perfSelectors,
			netproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			strconv.Itoa(numClients),
			cacheType,
			autoscaleCache,
			dursToString(dur),
			rpsToString(rps),
			clientDelay.String(),
		)
	}
}
