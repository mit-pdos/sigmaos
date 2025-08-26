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
	dialproxy := ""
	if bcfg.NoNetproxy {
		dialproxy = "--nodialproxy"
	}
	overlays := ""
	if bcfg.Overlays {
		overlays = "--overlays"
	}
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/sigmaclnt/fslib -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
		"--run InitFs "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		dialproxy,
		overlays,
		ccfg.LeaderNodeIP,
		bcfg.Tag,
	)
}

func GetExampleCmdConstructor(prewarm bool, exampleFlag string) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors    string = "\"TEST;THROUGHPUT;CPU_UTIL;SPAWN_LAT;PROXY_LAT;\""
			valgrindSelectors string = ""
			perfSelectors     string = "\"TEST_TPT;BENCH_TPT;\""
		)
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		prewarmStr := ""
		if prewarm {
			prewarmStr = "--prewarm_realm"
		}
		testName := "TestExample"
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAVALGRIND=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"ulimit -n 100000; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s --etcdIP %s --tag %s "+
			"--run %s "+
			"--example_flag %s "+
			"%s "+ // prewarm
			"> /tmp/bench.out 2>&1 ;",
			debugSelectors,
			valgrindSelectors,
			perfSelectors,
			dialproxy,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			testName,
			exampleFlag,
			prewarmStr,
		)
	}
}

func GetStartCmdConstructor(rps int, dur time.Duration, dummyProc, lcProc, prewarmRealm, skipStats bool) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors string = "\"TEST;BENCH;LOADGEN;\""
		)
		proc := "--use_rust_proc"
		if dummyProc {
			proc = "--use_dummy_proc"
		}
		lc := ""
		if lcProc {
			lc = "--spawn_bench_lc_proc"
		}
		prewarm := ""
		if prewarmRealm {
			prewarm = "--prewarm_realm"
		}
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		skipStatsPrint := ""
		if skipStats {
			skipStatsPrint = "--skipstats"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run TestMicroMSchedSpawn "+
			"%s "+ // proc
			"--nclnt 50 "+
			"%s "+ // skipStats
			"--msched_dur %s "+
			"--msched_max_rps %s "+
			"%s "+ // prewarmRealm
			"%s "+ // lcProc
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			dialproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			proc,
			skipStatsPrint,
			dur.String(),
			strconv.Itoa(rps),
			prewarm,
			lc,
		)
	}
}

// Construct command string to run BE imgresize multiplexing benchmark
func GetBEImgResizeMultiplexingCmd(bcfg *BenchConfig, ccfg *ClusterConfig) string {
	const (
		debugSelectors string = "\"TEST;BENCH;\""
	)
	dialproxy := ""
	if bcfg.NoNetproxy {
		dialproxy = "--nodialproxy"
	}
	overlays := ""
	if bcfg.Overlays {
		overlays = "--overlays"
	}
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
		"--run TestRealmBalanceImgResizeImgResize "+
		"--sleep 15s "+
		"--n_imgresize 10 "+
		"--imgresize_nround 300 "+
		"--n_imgresize_per 25 "+
		"--imgresize_path name/ux/~local/8.jpg "+
		"--imgresize_mcpu 0 "+
		"--imgresize_mem 1500 "+
		"--nrealm 4 "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		dialproxy,
		overlays,
		ccfg.LeaderNodeIP,
		bcfg.Tag,
	)
}

// Construct command string to run BE imgresize multiplexing benchmark
func GetBEImgResizeRPCMultiplexingCmd(bcfg *BenchConfig, ccfg *ClusterConfig) string {
	const (
		debugSelectors string = "\"TEST;BENCH;IMGD;\""
	)
	dialproxy := ""
	if bcfg.NoNetproxy {
		dialproxy = "--nodialproxy"
	}
	overlays := ""
	if bcfg.Overlays {
		overlays = "--overlays"
	}
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
		"--run TestRealmBalanceImgResizeRPCImgResizeRPC "+
		"--sleep 10s "+
		"--imgresize_tps 500 "+
		"--imgresize_dur 20s "+
		"--imgresize_nround 43 "+
		"--imgresize_path name/ux/~local/8.jpg "+
		"--imgresize_mcpu 0 "+
		"--imgresize_mem 2500 "+
		"--nrealm 4 "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		dialproxy,
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
func GetMRCmdConstructor(mrApp string, memReq proc.Tmem, prewarmRealm, measureTpt bool, perf bool) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors        string = "\"TEST;BENCH;MR_COORD\""
			optionalPerfSelectors string = "\"TEST_TPT;BENCH_TPT;\""
		)
		// If measuring throughput, set the perf selectors
		perfSelectors := "\"\""
		if measureTpt {
			perfSelectors = optionalPerfSelectors
		}
		if perf {
			perfSelectors = "\"MRREDUCER_PPROF;MRCOORD_PPROF;MRREDUCER_PPROF_MEM\""
		}
		prewarm := ""
		if prewarmRealm {
			prewarm = "--prewarm_realm"
		}
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"aws s3 rm --profile sigmaos --recursive s3://9ps3/mr-intermediate > /dev/null; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run AppMR "+
			"%s "+ // prewarm
			"--mr_mem_req %s "+
			"--mrapp %s "+
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			perfSelectors,
			dialproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			prewarm,
			strconv.Itoa(int(memReq)),
			mrApp,
		)
	}
}

// Construct command string to run corral benchmark.
func GetCorralCmdConstructor() GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		return "cd ../corral; " +
			"git pull; " +
			"git checkout play-perf-asynch; " +
			"git pull; " +
			// Load AWS key, because Corral expects this to be set as the default profile
			"export AWS_ACCESS_KEY_ID=$(cat ~/.aws/credentials | grep aws_access_key_id | head -n1 | cut -d ' ' -f3); " +
			"export AWS_SECRET_ACCESS_KEY=$(cat ~/.aws/credentials | grep aws_secret_access_key | head -n1 | cut -d ' ' -f3); " +
			"cd examples/word_count; " +
			"make test_wc_lambda " +
			"> /tmp/bench.out 2>&1"
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
// cached vs memcached).
//
// - If scaleCache is true, the cache autoscales.
//
// - clientDelay specifies the delay for which the client should wait before
// starting to send requests.
func GetHotelClientCmdConstructor(hotelReqName string, leader bool, numClients int, rps []int, dur []time.Duration, numCaches int, cacheType string, scaleCache bool, clientDelay time.Duration, manuallyScaleCaches bool, scaleCacheDelay time.Duration, numCachesToAdd int, numGeo int, geoNIdx int, geoSearchRadius int, geoNResults int, manuallyScaleGeo bool, scaleGeoDelay time.Duration, numGeoToAdd int) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			//			debugSelectors string = "\"TEST;THROUGHPUT;CPU_UTIL;\""
			debugSelectors string = "\"TEST;THROUGHPUT;CPU_UTIL;SPAWN_LAT\"" // XXX REMOVE
			perfSelectors  string = "\"HOTEL_WWW_TPT;TEST_TPT;BENCH_TPT;\""
			//			perfSelectors  string = "\"HOTEL_WWW_TPT;\"" // XXX Used to be just HOTEL_WWW_TPT. Is adding the others problematic?
		)
		sys := ""
		if bcfg.K8s {
			sys = "K8s"
		} else {
			sys = "Sigmaos"
		}
		testName := ""
		if leader {
			testName = fmt.Sprintf("Hotel%s%s", sys, hotelReqName)
		} else {
			testName = fmt.Sprintf("Hotel%sJustCli%s", sys, hotelReqName)
		}
		autoscaleCache := ""
		if scaleCache {
			autoscaleCache = "--hotel_cache_autoscale"
		}
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		k8sFrontendAddr := ""
		k8sFrontendLogScrapeCmd := "echo 'no scraping k8s logs'"
		if bcfg.K8s {
			addr, err := getK8sHotelFrontendAddr(bcfg, ccfg.lcfg)
			if err != nil {
				db.DFatalf("Get k8s hotel frontend addr:%v", err)
			}
			k8sFrontendAddr = fmt.Sprintf("--k8saddr %s", addr)
			if leader {
				k8sFrontendLogScrapeCmd = "kubectl logs service/frontend"
			}
		}
		scalecache := ""
		if manuallyScaleCaches {
			scalecache = "--manually_scale_caches"
		}
		scalegeo := ""
		if manuallyScaleGeo {
			scalegeo = "--manually_scale_geo"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"aws s3 rm --profile sigmaos --recursive s3://9ps3/hotelperf/k8s > /dev/null; "+
			"ulimit -n 100000; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run %s "+
			"--nclnt %s "+
			"--hotel_ncache %s "+
			"--hotel_cache_mcpu 2000 "+
			"--cache_type %s "+
			"%s "+ // scaleCache
			"%s "+ // k8sFrontendAddr
			"--hotel_dur %s "+
			"--hotel_max_rps %s "+
			"--sleep %s "+
			"%s "+ // manually_scale_caches
			"--scale_cache_delay %s "+
			"--n_caches_to_add %s "+
			"--hotel_ngeo %s "+
			"--hotel_ngeo_idx %s "+
			"--hotel_geo_search_radius %s "+
			"--hotel_geo_nresults %s "+
			"%s "+ // manually_scale_geo
			"--scale_geo_delay %s "+
			"--n_geo_to_add %s "+
			"--prewarm_realm "+
			"> /tmp/bench.out 2>&1 ; "+
			"%s > /tmp/frontend-logs.out 2>&1 ;",
			debugSelectors,
			perfSelectors,
			dialproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			testName,
			strconv.Itoa(numClients),
			strconv.Itoa(numCaches),
			cacheType,
			autoscaleCache,
			k8sFrontendAddr,
			dursToString(dur),
			rpsToString(rps),
			clientDelay.String(),
			scalecache,
			scaleCacheDelay.String(),
			strconv.Itoa(numCachesToAdd),
			strconv.Itoa(numGeo),
			strconv.Itoa(geoNIdx),
			strconv.Itoa(geoSearchRadius),
			strconv.Itoa(geoNResults),
			scalegeo,
			scaleGeoDelay.String(),
			strconv.Itoa(numGeoToAdd),
			k8sFrontendLogScrapeCmd,
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
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"aws s3 rm --profile sigmaos --recursive s3://9ps3/hotelperf/k8s > /dev/null; "+
			"ulimit -n 100000; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
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
			dialproxy,
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
// cached vs memcached).
//
// - If scaleCache is true, the cache autoscales.
//
// - sleep specifies the amount of time the hotel benchmark should sleep before
// starting to run.
func GetLCBEHotelImgResizeMultiplexingCmdConstructor(numClients int, rps []int, dur []time.Duration, cacheType string, scaleCache bool, sleep time.Duration) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors string = "\"TEST;BENCH;CPU_UTIL;IMGD;GROUPMGR;\""
			perfSelectors  string = "\"THUMBNAIL_TPT;TEST_TPT;BENCH_TPT;HOTEL_WWW_TPT;\""
		)
		autoscaleCache := ""
		if scaleCache {
			autoscaleCache = "--hotel_cache_autoscale"
		}
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"ulimit -n 100000; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run RealmBalanceHotelImgResize "+
			"--nclnt %s "+
			"--hotel_ncache 3 "+
			"--hotel_cache_mcpu 2000 "+
			"--cache_type %s "+
			"%s "+ // scaleCache
			"--hotel_dur %s "+
			"--hotel_max_rps %s "+
			"--sleep %s "+
			"--n_imgresize 350 "+
			"--imgresize_nround 500 "+
			"--n_imgresize_per 1 "+
			"--imgresize_path name/ux/~local/8.jpg "+
			"--imgresize_mcpu 0 "+
			"--imgresize_mem 1500 "+
			"--prewarm_realm "+
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			perfSelectors,
			dialproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			strconv.Itoa(numClients),
			cacheType,
			autoscaleCache,
			dursToString(dur),
			rpsToString(rps),
			sleep.String(),
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
// cached vs memcached).
//
// - If scaleCache is true, the cache autoscales.
//
// - sleep specifies the amount of time the hotel benchmark should sleep before
// starting to run.
func GetLCBEHotelImgResizeRPCMultiplexingCmdConstructor(numClients int, rps []int, dur []time.Duration, cacheType string, scaleCache bool, sleep time.Duration) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors string = "\"TEST;BENCH;CPU_UTIL;IMGD;GROUPMGR;\""
			perfSelectors  string = "\"THUMBNAIL_TPT;TEST_TPT;BENCH_TPT;HOTEL_WWW_TPT;\""
		)
		autoscaleCache := ""
		if scaleCache {
			autoscaleCache = "--hotel_cache_autoscale"
		}
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"ulimit -n 100000; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run RealmBalanceHotelRPCImgResize "+
			"--nclnt %s "+
			"--hotel_ncache 3 "+
			"--hotel_cache_mcpu 2000 "+
			"--cache_type %s "+
			"%s "+ // scaleCache
			"--hotel_dur %s "+
			"--hotel_max_rps %s "+
			"--sleep %s "+
			"--imgresize_tps 150 "+
			"--imgresize_dur 50s "+
			"--imgresize_nround 43 "+
			"--imgresize_path name/ux/~local/8.jpg "+
			"--imgresize_mcpu 0 "+
			"--imgresize_mem 2500 "+
			"--prewarm_realm "+
			"> /tmp/bench.out 2>&1",
			debugSelectors,
			perfSelectors,
			dialproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			strconv.Itoa(numClients),
			cacheType,
			autoscaleCache,
			dursToString(dur),
			rpsToString(rps),
			sleep.String(),
		)
	}
}

// Construct command string to run cosine similarity benchmark's load-generating client
//
// - numClients specifies the total number of client machines which will make
// requests to the cossim application
//
// - rps specifies the number of requests-per-second this client should execute
// in each phase of the benchmark.
//
// - dur specifies the duration for which each rps period should last.
//
// - If scaleCache is true, the cache autoscales.
//
// - clientDelay specifies the delay for which the client should wait before
// starting to send requests.
func GetCosSimClientCmdConstructor(cossimReqName string, leader bool, numClients int, rps []int, dur []time.Duration, numCaches int, scaleCache bool, clientDelay time.Duration, manuallyScaleCaches bool, scaleCacheDelay time.Duration, numCachesToAdd int, numCosSim int, nvec int, nvecToQuery int, vecDim int, cossimEagerInit, delegateInit, manuallyScaleCosSim bool, scaleCosSimDelay time.Duration, numCosSimToAdd int) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			//			debugSelectors string = "\"TEST;THROUGHPUT;CPU_UTIL;SPAWN_LAT;\""
			debugSelectors string = "\"TEST;THROUGHPUT;CPU_UTIL;SPAWN_LAT;PROXY_LAT;\""
			//			valgrindSelectors     string = "\"cossim-srv-cpp;\""
			valgrindSelectors string = ""
			perfSelectors     string = "\"COSSIMSRV_TPT;TEST_TPT;BENCH_TPT;\""
		)
		testName := ""
		if leader {
			testName = fmt.Sprintf("CosSim")
		} else {
			testName = fmt.Sprintf("CosSimJustCli")
		}
		autoscaleCache := ""
		if scaleCache {
			autoscaleCache = "--cossim_cache_autoscale"
		}
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		overlays := ""
		if bcfg.Overlays {
			overlays = "--overlays"
		}
		scalecache := ""
		if manuallyScaleCaches {
			scalecache = "--manually_scale_caches"
		}
		scaleCosSim := ""
		if manuallyScaleCosSim {
			scaleCosSim = "--manually_scale_cossim"
		}
		cossimDelegateInitStr := ""
		if delegateInit {
			cossimDelegateInitStr = "--cossim_delegated_init"
		}
		cossimEagerInitStr := ""
		if cossimEagerInit {
			cossimEagerInitStr = "--cossim_eager_init"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAVALGRIND=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"ulimit -n 100000; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s %s --etcdIP %s --tag %s "+
			"--run %s "+
			"--nclnt %s "+
			"--cossim_ncache %s "+
			"--cossim_cache_mcpu 2000 "+
			"--cossim_srv_mcpu 4000 "+
			"%s "+ // scaleCache
			"--cossim_dur %s "+
			"--cossim_max_rps %s "+
			"--sleep %s "+
			"%s "+ // manually_scale_caches
			"--scale_cache_delay %s "+
			"--n_caches_to_add %s "+
			"--ncossim %s "+
			"--cossim_nvec %s "+
			"--cossim_nvec_to_query %s "+
			"--cossim_vec_dim %s "+
			"%s "+ //cossim_eager_init
			"%s "+ //cossim_delegated_init
			"%s "+ // manually_scale_cossim
			"--scale_cossim_delay %s "+
			"--n_cossim_to_add %s "+
			"--prewarm_realm "+
			"> /tmp/bench.out 2>&1 ;",
			debugSelectors,
			valgrindSelectors,
			perfSelectors,
			dialproxy,
			overlays,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			testName,
			strconv.Itoa(numClients),
			strconv.Itoa(numCaches),
			autoscaleCache,
			dursToString(dur),
			rpsToString(rps),
			clientDelay.String(),
			scalecache,
			scaleCacheDelay.String(),
			strconv.Itoa(numCachesToAdd),
			strconv.Itoa(numCosSim),
			strconv.Itoa(nvec),
			strconv.Itoa(nvecToQuery),
			strconv.Itoa(vecDim),
			cossimEagerInitStr,
			cossimDelegateInitStr,
			scaleCosSim,
			scaleCosSimDelay.String(),
			strconv.Itoa(numCosSimToAdd),
		)
	}
}

func GetCachedBackupClientCmdConstructor(leader bool, numClients int, manuallyScaleBackupCached bool, scaleBackupCachedDelay time.Duration, rps []int, dur []time.Duration, putRps []int, putDur []time.Duration, clientDelay time.Duration, numCachedBackup int, nkeys int, topN int, delegateInit, useEPCache, prewarm bool) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors    string = "\"TEST;BENCH;LOADGEN;THROUGHPUT;CPU_UTIL;SPAWN_LAT;PROXY_LAT;\""
			valgrindSelectors string = ""
			perfSelectors     string = "\"CACHED_TPT;TEST_TPT;BENCH_TPT;\""
		)
		testName := ""
		if leader {
			testName = fmt.Sprintf("CachedBackup")
		} else {
			testName = fmt.Sprintf("CachedBackupJustCli")
		}
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		scaleBackupCachedStr := ""
		if manuallyScaleBackupCached {
			scaleBackupCachedStr = "--manually_scale_backup_cached"
		}
		cachedBackupUseEPCacheStr := ""
		if useEPCache {
			cachedBackupUseEPCacheStr = "--backup_cached_use_epcache"
		}
		delegateInitStr := ""
		if delegateInit {
			delegateInitStr = "--backup_cached_delegated_init"
		}
		prewarmStr := ""
		if prewarm {
			prewarmStr = "--prewarm_realm"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAVALGRIND=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"ulimit -n 100000; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s --etcdIP %s --tag %s "+
			"--run %s "+
			"--nclnt %s "+
			"--backup_cached_ncache %s "+
			"--backup_cached_mcpu 3000 "+
			"--backup_cached_dur %s "+
			"--backup_cached_max_rps %s "+
			"--backup_cached_put_dur %s "+
			"--backup_cached_put_max_rps %s "+
			"--sleep %s "+
			"--backup_cached_nkeys %s "+
			"--backup_cached_top_n %s "+
			"--scale_backup_cached_delay %s "+
			"%s "+ // manually_scale_backup_cached
			"%s "+ // backup_cached_delegated_init
			"%s "+ // cached_backup_use_epcache
			"%s "+ // prewarm
			"> /tmp/bench.out 2>&1 ;",
			debugSelectors,
			valgrindSelectors,
			perfSelectors,
			dialproxy,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			testName,
			strconv.Itoa(numClients),
			strconv.Itoa(numCachedBackup),
			dursToString(dur),
			rpsToString(rps),
			dursToString(putDur),
			rpsToString(putRps),
			clientDelay.String(),
			strconv.Itoa(nkeys),
			strconv.Itoa(topN),
			scaleBackupCachedDelay.String(),
			scaleBackupCachedStr,
			delegateInitStr,
			cachedBackupUseEPCacheStr,
			prewarmStr,
		)
	}
}

func GetCachedScalerClientCmdConstructor(leader bool, numClients int, manuallyScaleScalerCached bool, scaleScalerCachedDelay time.Duration, rps []int, dur []time.Duration, putRps []int, putDur []time.Duration, clientDelay time.Duration, numCachedScaler int, nkeys int, delegateInit, useEPCache, prewarm bool) GetBenchCmdFn {
	return func(bcfg *BenchConfig, ccfg *ClusterConfig) string {
		const (
			debugSelectors    string = "\"TEST;BENCH;LOADGEN;THROUGHPUT;CPU_UTIL;SPAWN_LAT;PROXY_LAT;\""
			valgrindSelectors string = ""
			perfSelectors     string = "\"CACHESRV_TPT;TEST_TPT;BENCH_TPT;\""
		)
		testName := ""
		if leader {
			testName = fmt.Sprintf("CachedScaler")
		} else {
			testName = fmt.Sprintf("CachedScalerJustCli")
		}
		dialproxy := ""
		if bcfg.NoNetproxy {
			dialproxy = "--nodialproxy"
		}
		scaleScalerCachedStr := ""
		if manuallyScaleScalerCached {
			scaleScalerCachedStr = "--manually_scale_scaler_cached"
		}
		cachedScalerUseEPCacheStr := ""
		if useEPCache {
			cachedScalerUseEPCacheStr = "--scaler_cached_use_epcache"
		}
		delegateInitStr := ""
		if delegateInit {
			delegateInitStr = "--scaler_cached_delegated_init"
		}
		prewarmStr := ""
		if prewarm {
			prewarmStr = "--prewarm_realm"
		}
		return fmt.Sprintf("export SIGMADEBUG=%s; export SIGMAVALGRIND=%s; export SIGMAPERF=%s; go clean -testcache; "+
			"ulimit -n 100000; "+
			"./set-cores.sh --set 1 --start 2 --end 39 > /dev/null 2>&1 ; "+
			"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown %s --etcdIP %s --tag %s "+
			"--run %s "+
			"--nclnt %s "+
			"--scaler_cached_ncache %s "+
			"--scaler_cached_mcpu 3000 "+
			"--scaler_cached_dur %s "+
			"--scaler_cached_max_rps %s "+
			"--scaler_cached_put_dur %s "+
			"--scaler_cached_put_max_rps %s "+
			"--sleep %s "+
			"--scaler_cached_nkeys %s "+
			"--scale_scaler_cached_delay %s "+
			"%s "+ // manually_scale_scaler_cached
			"%s "+ // scaler_cached_delegated_init
			"%s "+ // cached_scaler_use_epcache
			"%s "+ // prewarm
			"> /tmp/bench.out 2>&1 ;",
			debugSelectors,
			valgrindSelectors,
			perfSelectors,
			dialproxy,
			ccfg.LeaderNodeIP,
			bcfg.Tag,
			testName,
			strconv.Itoa(numClients),
			strconv.Itoa(numCachedScaler),
			dursToString(dur),
			rpsToString(rps),
			dursToString(putDur),
			rpsToString(putRps),
			clientDelay.String(),
			strconv.Itoa(nkeys),
			scaleScalerCachedDelay.String(),
			scaleScalerCachedStr,
			delegateInitStr,
			cachedScalerUseEPCacheStr,
			prewarmStr,
		)
	}
}
