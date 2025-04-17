package benchmarks_test

import (
	"flag"
	"math/rand"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/hotel"
	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	linuxsched "sigmaos/util/linux/sched"
	"sigmaos/util/perf"
	"strconv"
	"testing"
	"time"
)

const (
	REALM_BASENAME sp.Trealm = "benchrealm"
	//	REALM1                   = "arielck" // NOTE: Set this as realm name to take cold-start into account.
	REALM1 = REALM_BASENAME + "1"
	REALM2 = REALM_BASENAME + "2"

	MR_K8S_INIT_PORT int = 32585

	HOSTTMP string = "/tmp/"
)

// Parameters
var N_TRIALS int
var N_THREADS int
var PREWARM_REALM bool
var SKIPSTATS bool
var MR_APP string
var MR_MEM_REQ int
var KV_AUTO string
var N_KVD int
var N_CLERK int
var CLERK_DURATION string
var CLERK_MCPU int
var N_NODE_PER_MACHINE int
var N_CLNT int
var USE_RUST_PROC bool
var USE_DUMMY_PROC bool
var SPAWN_BENCH_LC_PROC bool
var WITH_KERNEL_PREF bool
var DOWNLOAD_FROM_UX bool
var MSCHED_DURS string
var MSCHED_MAX_RPS string
var N_CLNT_REQ int
var KVD_MCPU int
var WWWD_MCPU int
var WWWD_REQ_TYPE string
var WWWD_REQ_DELAY time.Duration
var HOTEL_NCACHE int
var HOTEL_NGEO int
var HOTEL_NGEO_IDX int
var HOTEL_GEO_SEARCH_RADIUS int
var HOTEL_GEO_NRESULTS int
var HOTEL_CACHE_MCPU int
var N_HOTEL int
var HOTEL_IMG_SZ_MB int
var HOTEL_CACHE_AUTOSCALE bool
var MANUALLY_SCALE_GEO bool
var MANUALLY_SCALE_CACHES bool
var N_GEO_TO_ADD int
var SCALE_GEO_DELAY time.Duration
var SCALE_CACHE_DELAY time.Duration
var N_CACHES_TO_ADD int
var CACHE_TYPE string
var CACHE_GC bool
var BLOCK_MEM string
var N_REALM int
var ASYNCRW bool

var MEMCACHED_ADDRS string
var HTTP_URL string
var DURATION time.Duration
var MAX_RPS int
var HOTEL_DURS string
var HOTEL_MAX_RPS string
var HOTEL_N_SPIN uint64
var SOCIAL_NETWORK_DURS string
var SOCIAL_NETWORK_MAX_RPS string
var SOCIAL_NETWORK_READ_ONLY bool
var IMG_RESIZE_INPUT_PATH string
var N_IMG_RESIZE_TASKS int
var IMG_RESIZE_DUR time.Duration
var N_IMG_RESIZE_TASKS_PER_SECOND int
var N_IMG_RESIZE_INPUTS_PER_TASK int
var IMG_RESIZE_MCPU int
var IMG_RESIZE_MEM_MB int
var IMG_RESIZE_N_ROUNDS int
var SLEEP time.Duration
var REDIS_ADDR string
var N_PROC int
var MCPU int
var MAT_SIZE int
var CONTENDERS_FRAC float64
var GO_MAX_PROCS int
var MAX_PARALLEL int
var K8S_ADDR string
var K8S_LEADER_NODE_IP string
var K8S_JOB_NAME string
var S3_RES_DIR string

// Read & set the proc version.
func init() {
	db.DPrintf(db.ALWAYS, "Benchmark Args: %v", os.Args)
	flag.IntVar(&N_REALM, "nrealm", 2, "Number of realms (relevant to BE balance benchmarks).")
	flag.IntVar(&N_TRIALS, "ntrials", 1, "Number of trials.")
	flag.IntVar(&N_THREADS, "nthreads", 1, "Number of threads.")
	flag.BoolVar(&PREWARM_REALM, "prewarm_realm", false, "Pre-warm realm, starting a BE and an LC uprocd on every machine in the cluster.")
	flag.BoolVar(&SKIPSTATS, "skipstats", false, "Skip printing stats.")
	flag.StringVar(&MR_APP, "mrapp", "mr-wc-wiki1.8G.yml", "Name of mr yaml file.")
	flag.IntVar(&MR_MEM_REQ, "mr_mem_req", 4000, "Amount of memory (in MB) required for each MR task.")
	flag.StringVar(&KV_AUTO, "kvauto", "manual", "KV auto-growing/shrinking.")
	flag.IntVar(&N_KVD, "nkvd", 1, "Number of kvds.")
	flag.IntVar(&N_CLERK, "nclerk", 1, "Number of clerks.")
	flag.IntVar(&N_CLNT, "nclnt", 1, "Number of clients.")
	flag.IntVar(&N_NODE_PER_MACHINE, "n_node_per_machine", 1, "Number of nodes per machine. Likely should always be 1, unless developing locally.")
	flag.BoolVar(&USE_RUST_PROC, "use_rust_proc", false, "Use rust spawn bench proc")
	flag.BoolVar(&USE_DUMMY_PROC, "use_dummy_proc", false, "Use dummy spawn bench proc")
	flag.BoolVar(&SPAWN_BENCH_LC_PROC, "spawn_bench_lc_proc", false, "Use an LC proc for spawn bench")
	flag.BoolVar(&WITH_KERNEL_PREF, "with_kernel_pref", false, "Set proc kernel preferences when spawning (e.g., to force & measure cold start)")
	flag.BoolVar(&DOWNLOAD_FROM_UX, "download_from_ux", false, "Download the proc from ux, instead of S3. !!! WARNING: this only works for the spawn-latency proc !!!")
	flag.StringVar(&MSCHED_DURS, "msched_dur", "10s", "MSched benchmark load generation duration (comma-separated for multiple phases).")
	flag.StringVar(&MSCHED_MAX_RPS, "msched_max_rps", "1000", "Max requests/second for msched bench (comma-separated for multiple phases).")
	flag.IntVar(&N_CLNT_REQ, "nclnt_req", 1, "Number of request each client news.")
	flag.StringVar(&CLERK_DURATION, "clerk_dur", "90s", "Clerk duration.")
	flag.IntVar(&CLERK_MCPU, "clerk_mcpu", 1000, "Clerk mCPU")
	flag.IntVar(&KVD_MCPU, "kvd_mcpu", 2000, "KVD mCPU")
	flag.IntVar(&WWWD_MCPU, "wwwd_mcpu", 2000, "WWWD mCPU")
	flag.StringVar(&WWWD_REQ_TYPE, "wwwd_req_type", "compute", "WWWD request type [compute, dummy, io].")
	flag.DurationVar(&WWWD_REQ_DELAY, "wwwd_req_delay", 500*time.Millisecond, "Average request delay.")
	flag.DurationVar(&SLEEP, "sleep", 0*time.Millisecond, "Sleep length.")
	flag.IntVar(&HOTEL_NCACHE, "hotel_ncache", 1, "Hotel ncache")
	flag.IntVar(&HOTEL_NGEO_IDX, "hotel_ngeo_idx", 1000, "Hotel num indexes per geo")
	flag.IntVar(&HOTEL_GEO_SEARCH_RADIUS, "hotel_geo_search_radius", 10, "Hotel geo search radius")
	flag.IntVar(&HOTEL_GEO_NRESULTS, "hotel_geo_nresults", 5, "Hotel num search results to return from geo")
	flag.IntVar(&HOTEL_NGEO, "hotel_ngeo", 1, "Hotel ngeo")
	flag.IntVar(&HOTEL_CACHE_MCPU, "hotel_cache_mcpu", 2000, "Hotel cache mcpu")
	flag.IntVar(&HOTEL_IMG_SZ_MB, "hotel_img_sz_mb", 0, "Hotel image data size in megabytes.")
	flag.Uint64Var(&HOTEL_N_SPIN, "hotel_n_spin_per_req", 0, "Number of spins per hotel spin request.")
	flag.IntVar(&N_HOTEL, "nhotel", 80, "Number of hotels in the dataset.")
	flag.BoolVar(&HOTEL_CACHE_AUTOSCALE, "hotel_cache_autoscale", false, "Autoscale hotel cache")
	flag.BoolVar(&MANUALLY_SCALE_GEO, "manually_scale_geo", false, "Manually scale geos")
	flag.DurationVar(&SCALE_GEO_DELAY, "scale_geo_delay", 0*time.Second, "Delay to wait before scaling up number of geos.")
	flag.IntVar(&N_GEO_TO_ADD, "n_geo_to_add", 0, "Number of geo to add.")
	flag.BoolVar(&MANUALLY_SCALE_CACHES, "manually_scale_caches", false, "Manually scale caches")
	flag.DurationVar(&SCALE_CACHE_DELAY, "scale_cache_delay", 0*time.Second, "Delay to wait before scaling up number of caches.")
	flag.IntVar(&N_CACHES_TO_ADD, "n_caches_to_add", 0, "Number of caches to add.")
	flag.StringVar(&CACHE_TYPE, "cache_type", "cached", "Hotel cache type (kvd or cached).")
	flag.BoolVar(&CACHE_GC, "cache_gc", false, "Turn hotel cache GC on (true) or off (false).")
	flag.StringVar(&BLOCK_MEM, "block_mem", "0MB", "Amount of physical memory to block on every machine.")
	flag.StringVar(&MEMCACHED_ADDRS, "memcached", "", "memcached server addresses (comma-separated).")
	flag.StringVar(&HTTP_URL, "http_url", "http://x.x.x.x", "HTTP url.")
	flag.DurationVar(&DURATION, "duration", 10*time.Second, "Duration.")
	flag.IntVar(&MAX_RPS, "max_rps", 1000, "Max requests per second.")
	flag.StringVar(&HOTEL_DURS, "hotel_dur", "10s", "Hotel benchmark load generation duration (comma-separated for multiple phases).")
	flag.StringVar(&HOTEL_MAX_RPS, "hotel_max_rps", "1000", "Max requests/second for hotel bench (comma-separated for multiple phases).")
	flag.StringVar(&SOCIAL_NETWORK_DURS, "sn_dur", "10s", "Social network benchmark load generation duration (comma-separated for multiple phases).")
	flag.StringVar(&SOCIAL_NETWORK_MAX_RPS, "sn_max_rps", "1000", "Max requests/second for social network bench (comma-separated for multiple phases).")
	flag.BoolVar(&SOCIAL_NETWORK_READ_ONLY, "sn_read_only", false, "send read only cases in social network bench")
	flag.StringVar(&K8S_ADDR, "k8saddr", "", "Kubernetes frontend service address (only for hotel benchmarking for the time being).")
	flag.StringVar(&K8S_LEADER_NODE_IP, "k8sleaderip", "", "Kubernetes leader node ip.")
	flag.StringVar(&K8S_JOB_NAME, "k8sjobname", "thumbnail-benchrealm1", "Name of k8s job")
	flag.StringVar(&S3_RES_DIR, "s3resdir", "", "Results dir in s3.")
	flag.StringVar(&REDIS_ADDR, "redisaddr", "", "Redis server address")
	flag.IntVar(&N_PROC, "nproc", 1, "Number of procs per trial.")
	flag.IntVar(&MCPU, "mcpu", 1000, "Generic proc test MCPU")
	flag.IntVar(&MAT_SIZE, "matrixsize", 4000, "Size of matrix.")
	flag.Float64Var(&CONTENDERS_FRAC, "contenders", 4000, "Fraction of cores which should be taken up by contending procs.")
	flag.IntVar(&GO_MAX_PROCS, "gomaxprocs", int(linuxsched.GetNCores()), "Go maxprocs setting for procs to be spawned.")
	flag.IntVar(&MAX_PARALLEL, "max_parallel", 1, "Max amount of parallelism.")
	flag.StringVar(&IMG_RESIZE_INPUT_PATH, "imgresize_path", "name/s3/"+sp.LOCAL+"/9ps3/img/1.jpg", "Path of img resize input file.")
	flag.IntVar(&N_IMG_RESIZE_TASKS, "n_imgresize", 10, "Number of img resize tasks.")
	flag.IntVar(&N_IMG_RESIZE_TASKS_PER_SECOND, "imgresize_tps", 1, "Number of img resize tasks/second.")
	flag.DurationVar(&IMG_RESIZE_DUR, "imgresize_dur", 10*time.Second, "Duration of imgresize job")
	flag.IntVar(&N_IMG_RESIZE_INPUTS_PER_TASK, "n_imgresize_per", 1, "Number of img resize inputs per job.")
	flag.IntVar(&IMG_RESIZE_MCPU, "imgresize_mcpu", 100, "MCPU for img resize worker.")
	flag.IntVar(&IMG_RESIZE_MEM_MB, "imgresize_mem", 0, "Mem for img resize worker.")
	flag.IntVar(&IMG_RESIZE_N_ROUNDS, "imgresize_nround", 1, "Number of rounds of computation for each image")

	db.DPrintf(db.ALWAYS, "ncore: %v", linuxsched.GetNCores())
}

// ========== Common parameters ==========
const (
	OUT_DIR = "name/out_dir"
)

func TestCompile(t *testing.T) {
}

// Test how long it takes to init a semaphore.
func TestMicroInitSemaphore(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(mrts.GetRealm(REALM1))
	_, is := newNSemaphores(mrts.GetRealm(REALM1), N_TRIALS)
	runOps(mrts.GetRealm(REALM1), is, initSemaphore, rs)
	printResultSummary(rs)
	rmOutDir(mrts.GetRealm(REALM1))
}

// Test how long it takes to up a semaphore.
func TestMicroUpSemaphore(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(mrts.GetRealm(REALM1))
	_, is := newNSemaphores(mrts.GetRealm(REALM1), N_TRIALS)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(mrts.GetRealm(REALM1), i)
	}
	runOps(mrts.GetRealm(REALM1), is, upSemaphore, rs)
	printResultSummary(rs)
	rmOutDir(mrts.GetRealm(REALM1))
}

// Test how long it takes to down a semaphore.
func TestMicroDownSemaphore(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(mrts.GetRealm(REALM1))
	_, is := newNSemaphores(mrts.GetRealm(REALM1), N_TRIALS)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(mrts.GetRealm(REALM1), i)
		upSemaphore(mrts.GetRealm(REALM1), i)
	}
	runOps(mrts.GetRealm(REALM1), is, downSemaphore, rs)
	printResultSummary(rs)
	rmOutDir(mrts.GetRealm(REALM1))
}

// Test how long it takes to cold Spawn and run the first instruction of
// hello-world proc
func TestMicroSpawnWaitStartRealm(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	if PREWARM_REALM {
		rs0 := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
		ps, is := newNProcs(N_TRIALS, "sleeper", []string{"1us", OUT_DIR}, nil, proc.Tmcpu(0))
		runOps(mrts.GetRealm(REALM1), is, spawnWaitStartProc, rs0)
		waitExitProcs(mrts.GetRealm(REALM1), ps)
	}

	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(mrts.GetRealm(REALM1))
	ps, is := newNProcs(N_TRIALS, "spawn-latency", []string{"1us", OUT_DIR}, nil, proc.Tmcpu(0))
	runOps(mrts.GetRealm(REALM1), is, spawnWaitStartProc, rs)
	waitExitProcs(mrts.GetRealm(REALM1), ps)
	db.DPrintf(db.BENCH, "Results:\n%v", rs)
	printResultSummary(rs)
	rmOutDir(mrts.GetRealm(REALM1))
}

// Test how long it takes to cold Spawn, run, and WaitExit the rust
// hello-world proc on a min node (i.e., without procq) that hasn't
// run any proc.
func TestMicroSpawnWaitStartNode(t *testing.T) {
	const N = 1
	// const PROG = "sleeper"
	const PROG = "spawn-latency"

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	sts, err := mrts.GetRoot().GetDir(sp.MSCHED)
	kernels := sp.Names(sts)
	kernel0 := kernels[0]

	err = mrts.GetRoot().BootMinNode(N)
	assert.Nil(t, err, "Boot node: %v", err)
	db.DPrintf(db.TEST, "Done boot node %d", N)

	sts, err = mrts.GetRoot().GetDir(sp.MSCHED)
	kernels = sp.Names(sts)
	kernel1 := ""
	for _, n := range kernels {
		if n != kernel0 {
			kernel1 = n
		}
	}
	db.DPrintf(db.TEST, "Kernel0 %v Kernel1 %v", kernel0, kernel1)

	if PREWARM_REALM {
		db.DPrintf(db.TEST, "prewarm: spawn on %v\n", kernel0)
		p := proc.NewProc(PROG, []string{"1us", OUT_DIR})
		p.SetKernels([]string{kernel0})
		err := mrts.GetRealm(REALM1).Spawn(p)
		assert.Nil(t, err, "Spawn")
		_, err = mrts.GetRealm(REALM1).WaitExit(p.GetPid())
		assert.Nil(t, err, "WaitExit")
	}

	time.Sleep(2 * time.Second)

	db.DPrintf(db.TEST, "cold spawn on %v\n", kernel1)
	s := time.Now()
	p := proc.NewProc(PROG, []string{"1us", OUT_DIR})
	p.SetKernels([]string{kernel1})
	err = mrts.GetRealm(REALM1).Spawn(p)
	assert.Nil(t, err, "Spawn")
	_, err = mrts.GetRealm(REALM1).WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit")
	db.DPrintf(db.BENCH, "Results: %v Cold start %v", kernel1, time.Since(s))
}

// Test how long it takes to Spawn, run, and WaitExit a 5ms proc.
func TestMicroSpawnWaitExit5msSleeper(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	if PREWARM_REALM {
		benchmarks.WarmupRealm(mrts.GetRealm(REALM1), []string{"sleeper"})
	}
	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(mrts.GetRealm(REALM1))
	_, ps := newNProcs(N_TRIALS, "sleeper", []string{"5000us", OUT_DIR}, nil, 1)
	runOps(mrts.GetRealm(REALM1), ps, runProc, rs)
	printResultSummary(rs)
	rmOutDir(mrts.GetRealm(REALM1))
}

// Test the throughput of spawning procs.
func TestMicroSpawnBurstTpt(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	db.DPrintf(db.ALWAYS, "SpawnBursting %v procs (ncore=%v) with max parallelism %v", N_PROC, MCPU, MAX_PARALLEL)
	ps, _ := newNProcs(N_PROC, "sleeper", []string{"0s", ""}, nil, proc.Tmcpu(MCPU))
	runOps(mrts.GetRealm(REALM1), []interface{}{ps}, spawnBurstWaitStartProcs, rs)
	printResultSummary(rs)
	waitExitProcs(mrts.GetRealm(REALM1), ps)
}

func TestMicrobenchmarksWarmupRealm(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(1, benchmarks.OPS)
	runOps(mrts.GetRealm(REALM1), []interface{}{"spawn-latency"}, warmupRealmBench, rs)
	printResultSummary(rs)
}

func TestMicroMSchedSpawn(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(1, benchmarks.OPS)

	db.DPrintf(db.BENCH, "rust %v ux %v dummy %v lc %v prewarm %v kpref %v nclnt %v durs %v rps %v skipstats %v", USE_RUST_PROC, DOWNLOAD_FROM_UX, USE_DUMMY_PROC, SPAWN_BENCH_LC_PROC, PREWARM_REALM, WITH_KERNEL_PREF, N_CLNT, MSCHED_DURS, MSCHED_MAX_RPS, SKIPSTATS)

	prog := "XXXX"
	if USE_RUST_PROC {
		if DOWNLOAD_FROM_UX {
			prog = "spawn-latency-ux"
		} else {
			prog = "spawn-latency"
		}
	} else if USE_DUMMY_PROC {
		assert.False(t, DOWNLOAD_FROM_UX, "Can only download rust proc from ux for now")
		prog = sp.DUMMY_PROG
	}

	sts, err := mrts.GetRoot().GetDir(sp.MSCHED)
	assert.Nil(mrts.GetRoot().T, err, "Err GetDir msched: %v", err)
	kernels := sp.Names(sts)
	db.DPrintf(db.TEST, "Kernels %v", kernels)

	// XXX clean up
	// Prep the sleeper bin cache
	db.DPrintf(db.TEST, "Warm up sleeper bin cache on kernel %v", kernels[0])
	p1 := proc.NewProc("sleeper", []string{"1ms", "name/"})
	p1.SetKernels([]string{kernels[0]})
	p1s := []*proc.Proc{p1}
	spawnProcs(mrts.GetRealm(REALM1), p1s)
	waitStartProcs(mrts.GetRealm(REALM1), p1s)
	waitExitProcs(mrts.GetRealm(REALM1), p1s)
	if PREWARM_REALM {
		db.DPrintf(db.TEST, "Warm up remainder of the realm for sleeper")
		benchmarks.WarmupRealm(mrts.GetRealm(REALM1), []string{"sleeper"})
	}
	if !USE_DUMMY_PROC {
		// Cold-start the first target proc to download the bin from S3
		db.DPrintf(db.TEST, "Warm up %v bin cache on kernel %v", prog, kernels[0])
		p2 := proc.NewProc(prog, nil)
		p2.SetKernels([]string{kernels[0]})
		p2s := []*proc.Proc{p2}
		spawnProcs(mrts.GetRealm(REALM1), p2s)
		waitStartProcs(mrts.GetRealm(REALM1), p2s)
		if PREWARM_REALM {
			benchmarks.WarmupRealm(mrts.GetRealm(REALM1), []string{prog})
		}
	}

	// Allow the uprocd pool to refill
	time.Sleep(5 * time.Second)

	done := make(chan bool)
	// Prep MSched job
	mschedJobs, ji := newMSchedJobs(mrts.GetRealm(REALM1), N_CLNT, MSCHED_DURS, MSCHED_MAX_RPS, prog, func(sc *sigmaclnt.SigmaClnt, pid sp.Tpid, kernelpref []string) time.Duration {
		if USE_RUST_PROC {
			return runRustSpawnBenchProc(mrts.GetRealm(REALM1), sc, prog, pid, kernelpref)
		} else if USE_DUMMY_PROC {
			return runDummySpawnBenchProc(mrts.GetRealm(REALM1), sc, pid, SPAWN_BENCH_LC_PROC)
		} else {
			return runSpawnBenchProc(mrts.GetRealm(REALM1), sc, pid, kernelpref)
		}
	}, kernels, WITH_KERNEL_PREF, SKIPSTATS)
	// Run MSched job
	go func() {
		runOps(mrts.GetRealm(REALM1), ji, runMSched, rs)
		done <- true
	}()
	// Wait for msched jobs to set up.
	<-mschedJobs[0].ready
	db.DPrintf(db.TEST, "MSched setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	// Sleep for a bit
	time.Sleep(SLEEP)
	// Kick off msched jobs
	mschedJobs[0].ready <- true
	<-done
	db.DPrintf(db.TEST, "MSched load bench done.")

	printResultSummary(rs)
}

// Test the throughput of spawning procs.
func TestMicroHTTPLoadGen(t *testing.T) {
	RunHTTPLoadGen(HTTP_URL, DURATION, MAX_RPS)
}

func TestAppMR(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	if PREWARM_REALM {
		benchmarks.WarmupRealm(mrts.GetRealm(REALM1), []string{"mr-coord", "mr-m-grep", "mr-r-grep", "mr-m-wc", "mr-r-wc"})
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(mrts.GetRealm(REALM1))
	defer p.Done()
	jobs, apps := newNMRJobs(mrts.GetRealm(REALM1), p, 1, MR_APP, chooseMRJobRoot(mrts.GetRealm(REALM1)), proc.Tmem(MR_MEM_REQ))
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	monitorCPUUtil(mrts.GetRealm(REALM1), p)
	runOps(mrts.GetRealm(REALM1), apps, runMR, rs)
	printResultSummary(rs)
}

func runKVTest(t *testing.T, nReplicas int) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(mrts.GetRealm(REALM1))
	defer p.Done()
	nclerks := []int{N_CLERK}
	db.DPrintf(db.ALWAYS, "Running with %v clerks", N_CLERK)
	jobs, ji := newNKVJobs(mrts.GetRealm(REALM1), 1, N_KVD, nReplicas, nclerks, nil, CLERK_DURATION, proc.Tmcpu(KVD_MCPU), proc.Tmcpu(CLERK_MCPU), KV_AUTO, REDIS_ADDR)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	monitorCPUUtil(mrts.GetRealm(REALM1), p)
	db.DPrintf(db.TEST, "runOps")
	runOps(mrts.GetRealm(REALM1), ji, runKV, rs)
	printResultSummary(rs)
}

func TestAppKVUnrepl(t *testing.T) {
	runKVTest(t, 0)
}

func TestAppKVRepl(t *testing.T) {
	runKVTest(t, 3)
}

func TestAppCached(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(mrts.GetRealm(REALM1))
	defer p.Done()
	const NKEYS = 100
	db.DPrintf(db.ALWAYS, "Running with %v clerks", N_CLERK)
	jobs, ji := newNCachedJobs(mrts.GetRealm(REALM1), 1, NKEYS, N_KVD, N_CLERK, CLERK_DURATION, proc.Tmcpu(CLERK_MCPU), proc.Tmcpu(KVD_MCPU))
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	monitorCPUUtil(mrts.GetRealm(REALM1), p)
	runOps(mrts.GetRealm(REALM1), ji, runCached, rs)
	printResultSummary(rs)
}

// Burst a bunch of spinning procs, and see how long it takes for all of them
// to start.
func TestRealmBurst(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ncores := countClusterCores(mrts.GetRoot()) - 1
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	newOutDir(mrts.GetRealm(REALM1))
	// Find the total number of cores available for spinners across all machines.
	// We need to get this in order to find out how many spinners to start.
	db.DPrintf(db.ALWAYS, "Bursting %v spinning procs", ncores)
	ps, _ := newNProcs(int(ncores), "spinner", []string{OUT_DIR}, nil, 1)
	p := newRealmPerf(mrts.GetRealm(REALM1))
	defer p.Done()
	monitorCPUUtil(mrts.GetRealm(REALM1), p)
	runOps(mrts.GetRealm(REALM1), []interface{}{ps}, spawnBurstWaitStartProcs, rs)
	printResultSummary(rs)
	evictProcs(mrts.GetRealm(REALM1), ps)
	rmOutDir(mrts.GetRealm(REALM1))
}

func TestLambdaBurst(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(1, benchmarks.E2E)
	newOutDir(mrts.GetRealm(REALM1))
	N_LAMBDAS := 720
	db.DPrintf(db.ALWAYS, "Invoking %v lambdas", N_LAMBDAS)
	ss, is := newNSemaphores(mrts.GetRealm(REALM1), N_LAMBDAS)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(mrts.GetRealm(REALM1), i)
	}
	runOps(mrts.GetRealm(REALM1), []interface{}{ss}, invokeWaitStartLambdas, rs)
	printResultSummary(rs)
	rmOutDir(mrts.GetRealm(REALM1))
}

func TestLambdaInvokeWaitStart(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(720, benchmarks.E2E)
	newOutDir(mrts.GetRealm(REALM1))
	N_LAMBDAS := 640
	db.DPrintf(db.ALWAYS, "Invoking %v lambdas", N_LAMBDAS)
	_, is := newNSemaphores(mrts.GetRealm(REALM1), N_LAMBDAS)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(mrts.GetRealm(REALM1), i)
	}
	runOps(mrts.GetRealm(REALM1), is, invokeWaitStartOneLambda, rs)
	printResultSummary(rs)
	rmOutDir(mrts.GetRealm(REALM1))
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceMRHotel(t *testing.T) {
	done := make(chan bool)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM2, REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	blockers := blockMem(mrts.GetRoot(), BLOCK_MEM)
	// Structures for mr
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p1.Done()
	// Structure for hotel
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p2.Done()
	// Prep MR job
	mrjobs, mrapps := newNMRJobs(mrts.GetRealm(REALM2), p1, 1, MR_APP, chooseMRJobRoot(mrts.GetRealm(REALM2)), proc.Tmem(MR_MEM_REQ))
	// Prep Hotel job
	hotelJobs, ji := newHotelJobs(mrts.GetRealm(REALM1), p2, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		//		hotel.RunDSB(mrts.GetRealm(REALM1).T, 1, wc, r)
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "SearchReq %v", err)
	})
	// Monitor cores assigned to MR.
	monitorCPUUtil(mrts.GetRealm(REALM2), p1)
	// Monitor cores assigned to Hotel.
	monitorCPUUtil(mrts.GetRealm(REALM1), p2)
	// Run Hotel job
	go func() {
		runOps(mrts.GetRealm(REALM1), ji, runHotel, rs2)
		done <- true
	}()
	// Wait for hotel jobs to set up.
	<-hotelJobs[0].ready
	db.DPrintf(db.TEST, "Hotel setup done.")
	// Run MR job
	go func() {
		runOps(mrts.GetRealm(REALM2), mrapps, runMR, rs1)
		done <- true
	}()
	// Wait for MR jobs to set up.
	<-mrjobs[0].ready
	db.DPrintf(db.TEST, "MR setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	if N_CLNT > 1 {
		// Wait for hotel clients to start up on other machines.
		db.DPrintf(db.ALWAYS, "Leader waiting for %v clnts", N_CLNT)
		waitForClnts(mrts.GetRoot(), N_CLNT)
		db.DPrintf(db.ALWAYS, "Leader done waiting for clnts")
	}
	db.DPrintf(db.TEST, "Done waiting for hotel clnts.")
	// Kick off MR jobs.
	mrjobs[0].ready <- true
	// Sleep for a bit
	time.Sleep(SLEEP)
	// Kick off hotel jobs
	hotelJobs[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	<-done
	db.DPrintf(db.TEST, "MR and Hotel done.")
	printResultSummary(rs1)
	time.Sleep(20 * time.Second)
	evictMemBlockers(mrts.GetRoot(), blockers)
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceHotelRPCImgResize(t *testing.T) {
	done := make(chan bool)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM2, REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	blockers := blockMem(mrts.GetRoot(), BLOCK_MEM)
	// Structures for imgresize
	//	mrts.GetRealm(REALM2), err1 := test.NewRealmTstate(mrts.GetRoot(), REALM2)
	//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
	//		return
	//	}
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p1.Done()
	if PREWARM_REALM {
		benchmarks.WarmupRealm(mrts.GetRealm(REALM2), []string{"imgresize", "imgresized"})
	}
	// Structure for hotel
	//	mrts.GetRealm(REALM1), err1 := test.NewRealmTstate(mrts.GetRoot(), REALM1)
	//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
	//		return
	//	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p2.Done()
	// Prep ImgResize job
	imgJobs, imgApps := newImgResizeRPCJob(mrts.GetRealm(REALM2), p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS_PER_SECOND, IMG_RESIZE_DUR, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
	// Prep Hotel job
	hotelJobs, ji := newHotelJobs(mrts.GetRealm(REALM1), p2, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		//		hotel.RunDSB(mrts.GetRealm(REALM1).T, 1, wc, r)
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "SearchReq %v", err)
	})
	// Monitor cores assigned to ImgResize.
	monitorCPUUtil(mrts.GetRealm(REALM2), p1)
	// Monitor cores assigned to Hotel.
	monitorCPUUtil(mrts.GetRealm(REALM1), p2)
	// Run Hotel job
	go func() {
		runOps(mrts.GetRealm(REALM1), ji, runHotel, rs2)
		done <- true
	}()
	// Wait for hotel jobs to set up.
	<-hotelJobs[0].ready
	db.DPrintf(db.TEST, "Hotel setup done.")
	// Run ImgResize job
	go func() {
		runOps(mrts.GetRealm(REALM2), imgApps, runImgResizeRPC, rs1)
		done <- true
	}()
	// Wait for imgResize jobs to set up.
	<-imgJobs[0].ready
	db.DPrintf(db.TEST, "Imgresize setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	if N_CLNT > 1 {
		// Wait for hotel clients to start up on other machines.
		db.DPrintf(db.ALWAYS, "Leader waiting for %v clnts", N_CLNT)
		waitForClnts(mrts.GetRoot(), N_CLNT)
		db.DPrintf(db.ALWAYS, "Leader done waiting for clnts")
	}
	db.DPrintf(db.TEST, "Done waiting for hotel clnts.")
	// Kick off ImgResize jobs.
	imgJobs[0].ready <- true
	// Sleep for a bit
	time.Sleep(SLEEP)
	// Kick off hotel jobs
	hotelJobs[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	<-done
	db.DPrintf(db.TEST, "Hotel and ImgResize done.")
	printResultSummary(rs1)
	time.Sleep(20 * time.Second)
	evictMemBlockers(mrts.GetRoot(), blockers)
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceHotelSpinRPCImgResize(t *testing.T) {
	done := make(chan bool)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM2, REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	blockers := blockMem(mrts.GetRoot(), BLOCK_MEM)
	// Structures for imgresize
	//	mrts.GetRealm(REALM2), err1 := test.NewRealmTstate(mrts.GetRoot(), REALM2)
	//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
	//		return
	//	}
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p1.Done()
	if PREWARM_REALM {
		benchmarks.WarmupRealm(mrts.GetRealm(REALM2), []string{"imgresize", "imgresized"})
	}
	// Structure for hotel
	//	mrts.GetRealm(REALM1), err1 := test.NewRealmTstate(mrts.GetRoot(), REALM1)
	//	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
	//		return
	//	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p2.Done()
	// Prep ImgResize job
	imgJobs, imgApps := newImgResizeRPCJob(mrts.GetRealm(REALM2), p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS_PER_SECOND, IMG_RESIZE_DUR, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
	// Prep Hotel job
	hotelJobs, ji := newHotelJobs(mrts.GetRealm(REALM1), p2, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		//		hotel.RunDSB(mrts.GetRealm(REALM1).T, 1, wc, r)
		_, err := hotel.SpinReq(wc, HOTEL_N_SPIN)
		assert.Nil(t, err, "SpinReq %v", err)
	})
	// Monitor cores assigned to ImgResize.
	monitorCPUUtil(mrts.GetRealm(REALM2), p1)
	// Monitor cores assigned to Hotel.
	monitorCPUUtil(mrts.GetRealm(REALM1), p2)
	// Run Hotel job
	go func() {
		runOps(mrts.GetRealm(REALM1), ji, runHotel, rs2)
		done <- true
	}()
	// Wait for hotel jobs to set up.
	<-hotelJobs[0].ready
	db.DPrintf(db.TEST, "Hotel setup done.")
	// Run ImgResize job
	go func() {
		runOps(mrts.GetRealm(REALM2), imgApps, runImgResizeRPC, rs1)
		done <- true
	}()
	// Wait for imgResize jobs to set up.
	<-imgJobs[0].ready
	db.DPrintf(db.TEST, "Imgresize setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	if N_CLNT > 1 {
		// Wait for hotel clients to start up on other machines.
		db.DPrintf(db.ALWAYS, "Leader waiting for %v clnts", N_CLNT)
		waitForClnts(mrts.GetRoot(), N_CLNT)
		db.DPrintf(db.ALWAYS, "Leader done waiting for clnts")
	}
	db.DPrintf(db.TEST, "Done waiting for hotel clnts.")
	// Kick off ImgResize jobs.
	imgJobs[0].ready <- true
	// Sleep for a bit
	time.Sleep(SLEEP)
	// Kick off hotel jobs
	hotelJobs[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	<-done
	db.DPrintf(db.TEST, "Hotel and ImgResize done.")
	printResultSummary(rs1)
	time.Sleep(20 * time.Second)
	evictMemBlockers(mrts.GetRoot(), blockers)
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceHotelImgResize(t *testing.T) {
	done := make(chan bool)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM2, REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	blockers := blockMem(mrts.GetRoot(), BLOCK_MEM)
	// Structures for imgresize
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p1.Done()
	if PREWARM_REALM {
		benchmarks.WarmupRealm(mrts.GetRealm(REALM2), []string{"imgresize", "imgresized"})
	}
	// Structure for hotel
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p2.Done()
	// Prep ImgResize job
	imgJobs, imgApps := newImgResizeJob(mrts.GetRealm(REALM2), p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, 0)
	// Prep Hotel job
	hotelJobs, ji := newHotelJobs(mrts.GetRealm(REALM1), p2, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		//		hotel.RunDSB(mrts.GetRealm(REALM1).T, 1, wc, r)
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "SearchReq %v", err)
	})
	// Monitor cores assigned to ImgResize.
	monitorCPUUtil(mrts.GetRealm(REALM2), p1)
	// Monitor cores assigned to Hotel.
	monitorCPUUtil(mrts.GetRealm(REALM1), p2)
	// Run Hotel job
	go func() {
		runOps(mrts.GetRealm(REALM1), ji, runHotel, rs2)
		done <- true
	}()
	// Wait for hotel jobs to set up.
	<-hotelJobs[0].ready
	db.DPrintf(db.TEST, "Hotel setup done.")
	// Run ImgResize job
	go func() {
		runOps(mrts.GetRealm(REALM2), imgApps, runImgResize, rs1)
		done <- true
	}()
	// Wait for imgResize jobs to set up.
	<-imgJobs[0].ready
	db.DPrintf(db.TEST, "Imgresize setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	if N_CLNT > 1 {
		// Wait for hotel clients to start up on other machines.
		db.DPrintf(db.ALWAYS, "Leader waiting for %v clnts", N_CLNT)
		waitForClnts(mrts.GetRoot(), N_CLNT)
		db.DPrintf(db.ALWAYS, "Leader done waiting for clnts")
	}
	db.DPrintf(db.TEST, "Done waiting for hotel clnts.")
	// Kick off ImgResize jobs.
	imgJobs[0].ready <- true
	// Sleep for a bit
	time.Sleep(SLEEP)
	// Kick off hotel jobs
	hotelJobs[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	<-done
	db.DPrintf(db.TEST, "Hotel and ImgResize done.")
	printResultSummary(rs1)
	time.Sleep(20 * time.Second)
	evictMemBlockers(mrts.GetRoot(), blockers)
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceMRMR(t *testing.T) {
	done := make(chan bool)
	realms := []sp.Trealm{}
	for i := 0; i < N_REALM; i++ {
		realms = append(realms, sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)))
	}
	mrts, err1 := test.NewMultiRealmTstate(t, realms)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	rses := make([]*benchmarks.Results, N_REALM)
	ps := make([]*perf.Perf, N_REALM)
	mrjobs := make([][]*MRJobInstance, N_REALM)
	mrapps := make([][]interface{}, N_REALM)
	// Create structures for MR jobs.
	for i := range realms {
		rses[i] = benchmarks.NewResults(1, benchmarks.E2E)
		ps[i] = newRealmPerf(mrts.GetRealm(realms[i]))
		defer ps[i].Done()
		mrjob, mrapp := newNMRJobs(mrts.GetRealm(realms[i]), ps[i], 1, MR_APP, chooseMRJobRoot(mrts.GetRealm(realms[i])), proc.Tmem(MR_MEM_REQ))
		mrjobs[i] = mrjob
		mrapps[i] = mrapp
	}
	// Start CPU utilization monitoring.
	for i := range realms {
		monitorCPUUtil(mrts.GetRealm(realms[i]), ps[i])
	}
	// Initialize MR jobs.
	for i := range realms {
		// Start MR job initialization.
		go func(ts *test.RealmTstate, mrapp []interface{}, rs *benchmarks.Results) {
			runOps(ts, mrapp, runMR, rs)
			done <- true
		}(mrts.GetRealm(realms[i]), mrapps[i], rses[i])
		// Wait for MR job to set up.
		<-mrjobs[i][0].ready
	}
	// Start jobs running, with a small delay between each job start.
	for i := range realms {
		// Kick off MR jobs.
		mrjobs[i][0].ready <- true
		db.DPrintf(db.TEST, "Start MR job %v", i+1)
		// Sleep for a bit before starting the next job
		time.Sleep(SLEEP)
	}
	// Wait for both jobs to finish.
	for i := range realms {
		<-done
		db.DPrintf(db.TEST, "Done MR job %v", i+1)
	}
	printResultSummary(rses[0])
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceImgResizeRPCImgResizeRPC(t *testing.T) {
	done := make(chan bool)
	realms := []sp.Trealm{}
	for i := 0; i < N_REALM; i++ {
		realms = append(realms, sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)))
	}
	mrts, err1 := test.NewMultiRealmTstate(t, realms)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	rses := make([]*benchmarks.Results, N_REALM)
	ps := make([]*perf.Perf, N_REALM)
	imgjobs := make([][]*ImgResizeRPCJobInstance, N_REALM)
	imgapps := make([][]interface{}, N_REALM)
	// Create structures for imgresize jobs.
	for i := range realms {
		rses[i] = benchmarks.NewResults(1, benchmarks.E2E)
		ps[i] = newRealmPerf(mrts.GetRealm(realms[i]))
		defer ps[i].Done()
		imgjob, imgapp := newImgResizeRPCJob(mrts.GetRealm(realms[i]), ps[i], true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS_PER_SECOND, IMG_RESIZE_DUR, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
		imgjobs[i] = imgjob
		imgapps[i] = imgapp
	}
	// Start CPU utilization monitoring.
	for i := range realms {
		monitorCPUUtil(mrts.GetRealm(realms[i]), ps[i])
	}
	// Initialize ImgResizeRPC jobs.
	for i := range realms {
		// Start ImgResizeRPCjob initialization.
		go func(ts *test.RealmTstate, imgapp []interface{}, rs *benchmarks.Results) {
			runOps(ts, imgapp, runImgResizeRPC, rs)
			done <- true
		}(mrts.GetRealm(realms[i]), imgapps[i], rses[i])
		// Wait for ImgResizeRPC job to set up.
		<-imgjobs[i][0].ready
	}
	// Start jobs running, with a small delay between each job start.
	for i := range realms {
		// Kick off ImgResizeRPC jobs.
		imgjobs[i][0].ready <- true
		db.DPrintf(db.TEST, "Start ImgResizeRPC job %v", i+1)
		// Sleep for a bit before starting the next job
		time.Sleep(SLEEP)
	}
	// Wait for both jobs to finish.
	for i := range realms {
		<-done
		db.DPrintf(db.TEST, "Done ImgResizeRPC job %v", i+1)
	}
	printResultSummary(rses[0])
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceImgResizeImgResize(t *testing.T) {
	done := make(chan bool)
	realms := []sp.Trealm{}
	for i := 0; i < N_REALM; i++ {
		realms = append(realms, sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)))
	}
	mrts, err1 := test.NewMultiRealmTstate(t, realms)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	rses := make([]*benchmarks.Results, N_REALM)
	ps := make([]*perf.Perf, N_REALM)
	imgjobs := make([][]*ImgResizeJobInstance, N_REALM)
	imgapps := make([][]interface{}, N_REALM)
	// Create structures for imgresize jobs.
	for i := range realms {
		rses[i] = benchmarks.NewResults(1, benchmarks.E2E)
		ps[i] = newRealmPerf(mrts.GetRealm(realms[i]))
		defer ps[i].Done()
		imgjob, imgapp := newImgResizeJob(mrts.GetRealm(realms[i]), ps[i], true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
		imgjobs[i] = imgjob
		imgapps[i] = imgapp
	}
	// Start CPU utilization monitoring.
	for i := range realms {
		monitorCPUUtil(mrts.GetRealm(realms[i]), ps[i])
	}
	// Initialize ImgResize jobs.
	for i := range realms {
		// Start ImgResize job initialization.
		go func(ts *test.RealmTstate, imgapp []interface{}, rs *benchmarks.Results) {
			runOps(ts, imgapp, runImgResize, rs)
			done <- true
		}(mrts.GetRealm(realms[i]), imgapps[i], rses[i])
		// Wait for ImgResize job to set up.
		<-imgjobs[i][0].ready
	}
	// Start jobs running, with a small delay between each job start.
	for i := range realms {
		// Kick off ImgResize jobs.
		imgjobs[i][0].ready <- true
		db.DPrintf(db.TEST, "Start ImgResize job %v", i+1)
		// Sleep for a bit before starting the next job
		time.Sleep(SLEEP)
	}
	// Wait for both jobs to finish.
	for i := range realms {
		<-done
		db.DPrintf(db.TEST, "Done ImgResize job %v", i+1)
	}
	printResultSummary(rses[0])
}

// Old realm balance benchmark involving KV & MR.
// Start a realm with a long-running BE mr job. Then, start a realm with a kv
// job. In phases, ramp the kv job's CPU utilization up and down, and watch the
// realm-level software balance resource requests across realms.
func TestKVMRRRB(t *testing.T) {
	done := make(chan bool)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1, REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	// Structures for mr
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p1.Done()
	// Structure for kv
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p2.Done()
	// Prep MR job
	mrjobs, mrapps := newNMRJobs(mrts.GetRealm(REALM1), p1, 1, MR_APP, chooseMRJobRoot(mrts.GetRealm(REALM1)), proc.Tmem(MR_MEM_REQ))
	// Prep KV job
	nclerks := []int{N_CLERK}
	kvjobs, ji := newNKVJobs(mrts.GetRealm(REALM2), 1, N_KVD, 0, nclerks, nil, CLERK_DURATION, proc.Tmcpu(KVD_MCPU), proc.Tmcpu(CLERK_MCPU), KV_AUTO, REDIS_ADDR)
	monitorCPUUtil(mrts.GetRealm(REALM1), p1)
	monitorCPUUtil(mrts.GetRealm(REALM2), p2)
	// Run KV job
	go func() {
		runOps(mrts.GetRealm(REALM2), ji, runKV, rs2)
		done <- true
	}()
	// Wait for KV jobs to set up.
	<-kvjobs[0].ready
	// Run MR job
	go func() {
		runOps(mrts.GetRealm(REALM1), mrapps, runMR, rs1)
		done <- true
	}()
	// Wait for MR jobs to set up.
	<-mrjobs[0].ready
	// Kick off MR jobs.
	mrjobs[0].ready <- true
	// Sleep for a bit
	time.Sleep(70 * time.Second)
	// Kick off KV jobs
	kvjobs[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	<-done
	printResultSummary(rs1)
	printResultSummary(rs2)
}

func testWww(t *testing.T, sigmaos bool) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer func() {
		if sigmaos {
			mrts.Shutdown()
		}
	}()
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	db.DPrintf(db.ALWAYS, "Running with %d clients", N_CLNT)
	jobs, ji := newWwwJobs(mrts.GetRealm(REALM1), sigmaos, 1, proc.Tmcpu(WWWD_MCPU), WWWD_REQ_TYPE, N_TRIALS, N_CLNT, N_CLNT_REQ, WWWD_REQ_DELAY)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	if sigmaos {
		p := newRealmPerf(mrts.GetRealm(REALM1))
		defer p.Done()
		monitorCPUUtil(mrts.GetRealm(REALM1), p)
	}
	runOps(mrts.GetRealm(REALM1), ji, runWww, rs)
	printResultSummary(rs)
}

func TestWwwSigmaos(t *testing.T) {
	testWww(t, true)
}

func TestWwwK8s(t *testing.T) {
	testWww(t, false)
}

func testHotel(rootts *test.Tstate, ts1 *test.RealmTstate, p *perf.Perf, sigmaos bool, fn hotelFn) {
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	jobs, ji := newHotelJobs(ts1, p, sigmaos, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, fn)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			if N_CLNT > 1 {
				// Wait for clients to start up on other machines.
				db.DPrintf(db.ALWAYS, "Leader waiting for clnts")
				waitForClnts(rootts, N_CLNT)
				db.DPrintf(db.ALWAYS, "Leader done waiting for clnts")
			}
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	if sigmaos {
		p := newRealmPerf(ts1)
		defer p.Done()
		monitorCPUUtil(ts1, p)
	} else {
		p := newRealmPerf(ts1)
		defer p.Done()
		monitorK8sCPUUtil(ts1, p, "hotel", "")
	}
	runOps(ts1, ji, runHotel, rs)
	//	printResultSummary(rs)
	if sigmaos {
		//		rootts.Shutdown()
	} else {
		jobs[0].requestK8sStats()
	}
}

func testSocialNet(rootts *test.Tstate, ts1 *test.RealmTstate, p *perf.Perf, sigmaos bool) {
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	jobs, apps := newSocialNetworkJobs(ts1, p, sigmaos, SOCIAL_NETWORK_READ_ONLY, SOCIAL_NETWORK_DURS, SOCIAL_NETWORK_MAX_RPS, 3)
	done := make(chan bool)
	// Run social network job
	go func() {
		runOps(ts1, apps, runSocialNetwork, rs)
		done <- true
	}()
	// Wait for social network jobs to set up.
	<-jobs[0].ready
	db.DPrintf(db.TEST, "Social Network setup done.")
	monitorCPUUtil(ts1, p)
	db.DPrintf(db.TEST, "Image Resize setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	// Kick off social network jobs
	jobs[0].ready <- true
	// Wait for jobs to finish.
	<-done
	db.DPrintf(db.TEST, "Social Network Done.")
	printResultSummary(rs)
	time.Sleep(5 * time.Second)
	rootts.Shutdown()
}

func TestSocialNetSigmaos(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	p1 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p1.Done()
	testSocialNet(mrts.GetRoot(), mrts.GetRealm(REALM1), p1, true)
}

func TestSocialNetK8s(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	p1 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p1.Done()
	testSocialNet(mrts.GetRoot(), mrts.GetRealm(REALM1), p1, false)
}

func TestHotelSigmaosGeo(t *testing.T) {
	db.DPrintf(db.ALWAYS, "scaleGeo %v delay %v n2a %v", MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	testHotel(mrts.GetRoot(), mrts.GetRealm(REALM1), nil, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		_, err := hotel.GeoReq(wc)
		assert.Nil(t, err, "Error search req: %v", err)
	})
}

func TestHotelSigmaosSearch(t *testing.T) {
	db.DPrintf(db.ALWAYS, "scaleGeo %v delay %v n2a %v", MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	testHotel(mrts.GetRoot(), mrts.GetRealm(REALM1), nil, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "Error search req: %v", err)
	})
}

func TestHotelDevSigmaosSearchScaleCache(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	N := 3
	err := mrts.GetRoot().BootMinNode(N)
	assert.Nil(t, err, "Boot node: %v", err)
	db.DPrintf(db.TEST, "Done boot node %d", N)
	db.DPrintf(db.TEST, "Done boot node %d", N)
	testHotel(mrts.GetRoot(), mrts.GetRealm(REALM1), nil, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "Error search req: %v", err)
	})
}

func TestHotelSigmaosJustCliGeo(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if err1 := waitForRealmCreation(mrts.GetRoot(), REALM1); !assert.Nil(t, err1, "Error waitRealmCreation: %v") {
		return
	}
	if err1 := mrts.AddRealmClnt(REALM1); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	clientReady(mrts.GetRoot())
	// Sleep for a bit
	time.Sleep(SLEEP)
	jobs, ji := newHotelJobsCli(mrts.GetRealm(REALM1), true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		_, err := hotel.GeoReq(wc)
		assert.Nil(t, err, "Error geo req: %v", err)
	})
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	runOps(mrts.GetRealm(REALM1), ji, runHotel, rs)
	//	printResultSummary(rs)
	//	jobs[0].requestK8sStats()
}

func TestHotelSigmaosJustCliSpin(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if err1 := waitForRealmCreation(mrts.GetRoot(), REALM1); !assert.Nil(t, err1, "Error waitRealmCreation: %v") {
		return
	}
	if err1 := mrts.AddRealmClnt(REALM1); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	clientReady(mrts.GetRoot())
	// Sleep for a bit
	time.Sleep(SLEEP)
	jobs, ji := newHotelJobsCli(mrts.GetRealm(REALM1), true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		_, err := hotel.SpinReq(wc, HOTEL_N_SPIN)
		assert.Nil(t, err, "Error search req: %v", err)
	})
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	runOps(mrts.GetRealm(REALM1), ji, runHotel, rs)
	//	printResultSummary(rs)
	//	jobs[0].requestK8sStats()
}

func TestHotelSigmaosJustCliSearch(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if err1 := waitForRealmCreation(mrts.GetRoot(), REALM1); !assert.Nil(t, err1, "Error waitRealmCreation: %v") {
		return
	}
	if err1 := mrts.AddRealmClnt(REALM1); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	clientReady(mrts.GetRoot())
	// Sleep for a bit
	time.Sleep(SLEEP)
	jobs, ji := newHotelJobsCli(mrts.GetRealm(REALM1), true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "Error search req: %v", err)
	})
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	runOps(mrts.GetRealm(REALM1), ji, runHotel, rs)
	//	printResultSummary(rs)
	//	jobs[0].requestK8sStats()
}

func TestHotelK8sJustCliGeo(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if err1 := waitForRealmCreation(mrts.GetRoot(), REALM1); !assert.Nil(t, err1, "Error waitRealmCreation: %v") {
		return
	}
	if err1 := mrts.AddRealmClnt(REALM1); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	db.DPrintf(db.ALWAYS, "Clnt ready")
	clientReady(mrts.GetRoot())
	db.DPrintf(db.ALWAYS, "Clnt done waiting")
	jobs, ji := newHotelJobsCli(mrts.GetRealm(REALM1), false, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		_, err := hotel.GeoReq(wc)
		assert.Nil(t, err, "Error geo req: %v", err)
	})
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	runOps(mrts.GetRealm(REALM1), ji, runHotel, rs)
	//	printResultSummary(rs)
	//	jobs[0].requestK8sStats()
}

func TestHotelK8sJustCliSearch(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if err1 := waitForRealmCreation(mrts.GetRoot(), REALM1); !assert.Nil(t, err1, "Error waitRealmCreation: %v") {
		return
	}
	if err1 := mrts.AddRealmClnt(REALM1); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	db.DPrintf(db.ALWAYS, "Clnt ready")
	clientReady(mrts.GetRoot())
	db.DPrintf(db.ALWAYS, "Clnt done waiting")
	jobs, ji := newHotelJobsCli(mrts.GetRealm(REALM1), false, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, HOTEL_NGEO, MANUALLY_SCALE_GEO, SCALE_GEO_DELAY, N_GEO_TO_ADD, HOTEL_NGEO_IDX, HOTEL_GEO_SEARCH_RADIUS, HOTEL_GEO_NRESULTS, func(wc *hotel.WebClnt, r *rand.Rand) {
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "Error search req: %v", err)
	})
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	runOps(mrts.GetRealm(REALM1), ji, runHotel, rs)
	//	printResultSummary(rs)
	//	jobs[0].requestK8sStats()
}

func TestHotelK8sGeo(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	testHotel(mrts.GetRoot(), mrts.GetRealm(REALM1), nil, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		_, err := hotel.GeoReq(wc)
		assert.Nil(t, err, "Error geo req: %v", err)
	})
	downloadS3Results(mrts.GetRoot(), filepath.Join("name/s3/~any/9ps3/", "hotelperf/k8s"), HOSTTMP+"sigmaos-perf")
}

func TestHotelK8sSearch(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	testHotel(mrts.GetRoot(), mrts.GetRealm(REALM1), nil, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "Error search req: %v", err)
	})
	downloadS3Results(mrts.GetRoot(), filepath.Join("name/s3/~any/9ps3/", "hotelperf/k8s"), HOSTTMP+"sigmaos-perf")
}

func TestHotelSigmaosAll(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	testHotel(mrts.GetRoot(), mrts.GetRealm(REALM1), nil, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RunDSB(mrts.GetRoot().T, 1, wc, r)
	})
}

func TestHotelK8sAll(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	testHotel(mrts.GetRoot(), mrts.GetRealm(REALM1), nil, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RunDSB(mrts.GetRoot().T, 1, wc, r)
	})
}

func TestMRK8s(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	assert.NotEqual(mrts.GetRoot().T, K8S_LEADER_NODE_IP, "", "Must pass k8s leader node ip")
	assert.NotEqual(mrts.GetRoot().T, S3_RES_DIR, "", "Must pass s3 reulst dir")
	if K8S_LEADER_NODE_IP == "" || S3_RES_DIR == "" {
		db.DPrintf(db.ALWAYS, "Skipping mr k8s")
		return
	}
	c := startK8sMR(mrts.GetRoot(), k8sMRAddr(K8S_LEADER_NODE_IP, MR_K8S_INIT_PORT))
	waitK8sMR(mrts.GetRoot(), c)
	downloadS3Results(mrts.GetRoot(), filepath.Join("name/s3/~any/9ps3/", S3_RES_DIR), HOSTTMP+"sigmaos-perf")
}

func TestK8sMRMulti(t *testing.T) {
	realms := []sp.Trealm{}
	for i := 0; i < N_REALM; i++ {
		realms = append(realms, sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)))
	}
	mrts, err1 := test.NewMultiRealmTstate(t, realms)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	assert.NotEqual(mrts.GetRoot().T, K8S_LEADER_NODE_IP, "", "Must pass k8s leader node ip")
	assert.NotEqual(mrts.GetRoot().T, S3_RES_DIR, "", "Must pass s3 result dir")
	if K8S_LEADER_NODE_IP == "" || S3_RES_DIR == "" {
		db.DPrintf(db.ALWAYS, "Skipping mr k8s")
		return
	}
	// Create realm structures.
	ps := make([]*perf.Perf, 0, N_REALM)
	for i := 0; i < N_REALM; i++ {
		db.DPrintf(db.TEST, "Create realm srtructs for %v", realms[i])
		ps = append(ps, newRealmPerf(mrts.GetRealm(realms[i])))
		defer ps[i].Done()
	}
	db.DPrintf(db.TEST, "Done creating realm srtructs")
	err := mrts.GetRealm(realms[0]).MkDir(sp.K8S_SCRAPER, 0777)
	assert.Nil(mrts.GetRoot().T, err, "Error mkdir %v", err)
	// Start up the stat scraper procs.
	sdc := mschedclnt.NewMSchedClnt(mrts.GetRealm(realms[0]).SigmaClnt.FsLib, sp.NOT_SET)
	nMSched, err := sdc.NMSched()
	ps2, _ := newNProcs(nMSched, "k8s-stat-scraper", []string{}, nil, proc.Tmcpu(1000*(linuxsched.GetNCores()-1)))
	spawnBurstProcs(mrts.GetRealm(realms[0]), ps2)
	waitStartProcs(mrts.GetRealm(realms[0]), ps2)

	cs := make([]*rpc.Client, 0, N_REALM)
	for i := 0; i < N_REALM; i++ {
		rName := sp.Trealm(REALM_BASENAME.String() + strconv.Itoa(i+1))
		db.DPrintf(db.TEST, "Starting MR job for realm %v", rName)
		// Start the next k8s job.
		cs = append(cs, startK8sMR(mrts.GetRoot(), k8sMRAddr(K8S_LEADER_NODE_IP, MR_K8S_INIT_PORT+i+1)))
		// Monitor cores assigned to this realm.
		//		monitorK8sCPUUtil(ts[i], ps[i], "mr", rName)
		monitorK8sCPUUtilScraperTS(mrts.GetRealm(realms[0]), ps[i], "Guaranteed")
		// Sleep for a bit before starting the next job
		time.Sleep(SLEEP)
	}
	db.DPrintf(db.TEST, "Done starting MR jobs")
	for i, c := range cs {
		waitK8sMR(mrts.GetRoot(), c)
		db.DPrintf(db.TEST, "MR job %v finished", i)
	}
	db.DPrintf(db.TEST, "Done waiting for MR jobs.")
	for i := 0; i < N_REALM; i++ {
		downloadS3ResultsRealm(
			mrts.GetRoot(),
			filepath.Join("name/s3/~any/9ps3/", S3_RES_DIR+"-"+strconv.Itoa(i+1)),
			HOSTTMP+"sigmaos-perf",
			sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)),
		)
	}
	db.DPrintf(db.TEST, "Done downloading results.")
}

func TestK8sBalanceHotelMR(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM2, REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	// Structures for mr
	p1 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p1.Done()
	// Structure for hotel
	p2 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p2.Done()
	// Monitor cores assigned to MR.
	monitorK8sCPUUtil(mrts.GetRealm(REALM2), p1, "mr", "")
	// Monitor cores assigned to Hotel.
	monitorK8sCPUUtil(mrts.GetRealm(REALM1), p2, "hotel", "")
	assert.NotEqual(mrts.GetRoot().T, K8S_LEADER_NODE_IP, "", "Must pass k8s leader node ip")
	assert.NotEqual(mrts.GetRoot().T, S3_RES_DIR, "", "Must pass k8s leader node ip")
	db.DPrintf(db.TEST, "Starting hotel")
	done := make(chan bool)
	go func() {
		testHotel(mrts.GetRoot(), mrts.GetRealm(REALM1), nil, false, func(wc *hotel.WebClnt, r *rand.Rand) {
			hotel.RandSearchReq(wc, r)
		})
		done <- true
	}()
	db.DPrintf(db.TEST, "Starting mr")
	if K8S_LEADER_NODE_IP == "" || S3_RES_DIR == "" {
		db.DPrintf(db.ALWAYS, "Skipping mr k8s")
		return
	}
	c := startK8sMR(mrts.GetRoot(), k8sMRAddr(K8S_LEADER_NODE_IP, MR_K8S_INIT_PORT))
	waitK8sMR(mrts.GetRoot(), c)
	<-done
	db.DPrintf(db.TEST, "Downloading results")
	downloadS3Results(mrts.GetRoot(), filepath.Join("name/s3/~any/9ps3/", S3_RES_DIR), HOSTTMP+"sigmaos-perf")
	downloadS3Results(mrts.GetRoot(), filepath.Join("name/s3/~any/9ps3/", "hotelperf/k8s"), HOSTTMP+"sigmaos-perf")
}

func TestImgResize(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	if PREWARM_REALM {
		benchmarks.WarmupRealm(mrts.GetRealm(REALM1), []string{"imgresize", "imgresized"})
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(mrts.GetRealm(REALM1))
	defer p.Done()
	jobs, apps := newImgResizeJob(mrts.GetRealm(REALM1), p, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	monitorCPUUtil(mrts.GetRealm(REALM1), p)
	runOps(mrts.GetRealm(REALM1), apps, runImgResize, rs)
	printResultSummary(rs)
}

func TestK8sImgResize(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	if err1 := mrts.AddRealmClnt(REALM1); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if PREWARM_REALM {
		benchmarks.WarmupRealm(mrts.GetRealm(REALM1), nil)
	}
	sdc := mschedclnt.NewMSchedClnt(mrts.GetRealm(REALM1).FsLib, sp.NOT_SET)
	nMSched, err := sdc.NMSched()
	assert.Nil(mrts.GetRealm(REALM1).Ts.T, err, "Error nmsched %v", err)
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(mrts.GetRealm(REALM1))
	defer p.Done()
	err = mrts.GetRealm(REALM1).MkDir(sp.K8S_SCRAPER, 0777)
	assert.Nil(mrts.GetRealm(REALM1).Ts.T, err, "Error mkdir %v", err)
	// Start up the stat scraper procs.
	ps, _ := newNProcs(nMSched, "k8s-stat-scraper", []string{}, nil, proc.Tmcpu(1000*(linuxsched.GetNCores()-1)))
	spawnBurstProcs(mrts.GetRealm(REALM1), ps)
	waitStartProcs(mrts.GetRealm(REALM1), ps)
	// NOte start time
	start := time.Now()
	// Monitor CPU utilization via the stat scraper procs.
	monitorK8sCPUUtilScraper(mrts.GetRoot(), p, "BestEffort")
	exec.Command("kubectl", "apply", "-Rf", "/tmp/thumbnail.yaml").Start()
	for !k8sJobHasCompleted(K8S_JOB_NAME) {
		time.Sleep(500 * time.Millisecond)
	}
	rs.Append(time.Since(start), 1)
	printResultSummary(rs)
	evictProcs(mrts.GetRealm(REALM1), ps)
}

func TestRealmBalanceSimpleImgResize(t *testing.T) {
	done := make(chan bool)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1, REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	blockers := blockMem(mrts.GetRoot(), BLOCK_MEM)
	// Structures for BE image resize
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p1.Done()
	// Structure for LC image resize
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p2.Done()
	// Prep resize jobs
	imgJobsBE, imgAppsBE := newImgResizeJob(
		mrts.GetRealm(REALM1), p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, 0, proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
	imgJobsLC, imgAppsLC := newImgResizeJob(
		mrts.GetRealm(REALM2), p2, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))

	// Run image resize jobs
	go func() {
		runOps(mrts.GetRealm(REALM1), imgAppsBE, runImgResize, rs1)
		done <- true
	}()
	go func() {
		runOps(mrts.GetRealm(REALM2), imgAppsLC, runImgResize, rs2)
		done <- true
	}()
	// Wait for image resize jobs to set up.
	<-imgJobsBE[0].ready
	<-imgJobsLC[0].ready

	// Monitor cores for kernel procs
	monitorCPUUtil(mrts.GetRealm(REALM1), p1)
	monitorCPUUtil(mrts.GetRealm(REALM2), p2)
	db.DPrintf(db.TEST, "Image Resize setup done.")
	// Kick off image resize jobs.
	imgJobsBE[0].ready <- true
	// Sleep for a bit
	time.Sleep(5 * time.Second)
	// Kick off social network jobs
	imgJobsLC[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	<-done
	db.DPrintf(db.TEST, "Image Resize Done.")
	printResultSummary(rs1)
	evictMemBlockers(mrts.GetRoot(), blockers)
}

func TestRealmBalanceSocialNetworkImgResize(t *testing.T) {
	done := make(chan bool)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1, REALM2})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	blockers := blockMem(mrts.GetRoot(), BLOCK_MEM)
	if err1 := mrts.AddRealmClnt(sp.ROOTREALM); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p0 := newRealmPerf(mrts.GetRealm(sp.ROOTREALM))
	defer p0.Done()
	// Structures for image resize
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p1.Done()
	// Structure for social network
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p2.Done()
	// Prep image resize job
	imgJobs, imgApps := newImgResizeJob(
		mrts.GetRealm(REALM1), p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, 0, proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, 0)
	// Prep social network job
	snJobs, snApps := newSocialNetworkJobs(mrts.GetRealm(REALM2), p2, true, SOCIAL_NETWORK_READ_ONLY, SOCIAL_NETWORK_DURS, SOCIAL_NETWORK_MAX_RPS, 3)
	// Run social network job
	go func() {
		runOps(mrts.GetRealm(REALM2), snApps, runSocialNetwork, rs2)
		done <- true
	}()
	// Wait for social network jobs to set up.
	<-snJobs[0].ready
	db.DPrintf(db.TEST, "Social Network setup done.")
	// Run image resize job
	go func() {
		runOps(mrts.GetRealm(REALM1), imgApps, runImgResize, rs1)
		done <- true
	}()
	// Wait for image resize jobs to set up.
	<-imgJobs[0].ready
	// Monitor cores for kernel procs
	monitorCPUUtil(mrts.GetRealm(sp.ROOTREALM), p0)
	monitorCPUUtil(mrts.GetRealm(REALM1), p1)
	monitorCPUUtil(mrts.GetRealm(REALM2), p2)
	db.DPrintf(db.TEST, "Image Resize setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	// Kick off image resize jobs.
	imgJobs[0].ready <- true
	// Sleep for a bit
	time.Sleep(10 * time.Second)
	// Kick off social network jobs
	snJobs[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	<-done
	db.DPrintf(db.TEST, "Image Resize and Social Network Done.")
	printResultSummary(rs1)
	time.Sleep(5 * time.Second)
	evictMemBlockers(mrts.GetRoot(), blockers)
}

func TestK8sSocialNetworkImgResize(t *testing.T) {
	done := make(chan bool)
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()
	blockers := blockMem(mrts.GetRoot(), BLOCK_MEM)
	// make realm to run k8s scrapper
	if err1 := mrts.AddRealmClnt(sp.ROOTREALM); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p0 := newRealmPerf(mrts.GetRealm(sp.ROOTREALM))
	defer p0.Done()
	if PREWARM_REALM {
		benchmarks.WarmupRealm(mrts.GetRealm(sp.ROOTREALM), nil)
	}
	sdc := mschedclnt.NewMSchedClnt(mrts.GetRealm(sp.ROOTREALM).SigmaClnt.FsLib, sp.NOT_SET)
	nMSched, err := sdc.NMSched()
	assert.Nil(mrts.GetRealm(sp.ROOTREALM).Ts.T, err, "Error nmsched %v", err)
	rs0 := benchmarks.NewResults(1, benchmarks.E2E)
	err = mrts.GetRealm(sp.ROOTREALM).MkDir(sp.K8S_SCRAPER, 0777)
	assert.Nil(mrts.GetRealm(sp.ROOTREALM).Ts.T, err, "Error mkdir %v", err)
	// Start up the stat scraper procs.
	//ps, _ := newNProcs(nMSched, "k8s-stat-scraper", []string{}, nil, proc.Tmcpu(1000*(linuxsched.GetNCores()-1)))
	ps, _ := newNProcs(nMSched, "k8s-stat-scraper", []string{}, nil, 0)
	spawnBurstProcs(mrts.GetRealm(sp.ROOTREALM), ps)
	waitStartProcs(mrts.GetRealm(sp.ROOTREALM), ps)
	// Structures for image resize
	if err1 := mrts.AddRealmClnt(REALM1); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	//rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(mrts.GetRealm(REALM1))
	defer p1.Done()
	// Structure for social network
	if err1 := mrts.AddRealm(REALM2); !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(mrts.GetRealm(REALM2))
	defer p2.Done()
	// Prep image resize job

	// Prep social network job
	snJobs, snApps := newSocialNetworkJobs(mrts.GetRealm(REALM2), p2, false, SOCIAL_NETWORK_READ_ONLY, SOCIAL_NETWORK_DURS, SOCIAL_NETWORK_MAX_RPS, 3)
	// Monitor cores assigned to image resize.
	// NOte start time
	start := time.Now()
	// Run social network job
	go func() {
		runOps(mrts.GetRealm(REALM2), snApps, runSocialNetwork, rs2)
		snJobs[0].requestK8sStats()
		done <- true
	}()
	// Wait for social network jobs to set up.
	<-snJobs[0].ready
	db.DPrintf(db.TEST, "Social Network setup done.")
	// Monitor CPU utilization via the stat scraper procs.
	monitorK8sCPUUtilScraper(mrts.GetRoot(), p2, "Burstable")
	monitorK8sCPUUtilScraper(mrts.GetRoot(), p1, "BestEffort")
	// Run image resize job
	exec.Command("kubectl", "apply", "-Rf", "/tmp/thumbnail-heavy/").Start()
	// Wait for image resize jobs to set up.
	db.DPrintf(db.TEST, "Setup phase done.")
	// Kick off image resize jobs.
	// Sleep for a bit
	time.Sleep(5 * time.Second)
	// Kick off social network jobs
	snJobs[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	db.DPrintf(db.TEST, "Downloading results")
	downloadS3Results(mrts.GetRoot(), filepath.Join("name/s3/~any/9ps3/", "social-network-perf/k8s"), HOSTTMP+"sigmaos-perf")
	for !(k8sJobHasCompleted("thumbnail1-benchrealm1") && k8sJobHasCompleted("thumbnail2-benchrealm1") &&
		k8sJobHasCompleted("thumbnail3-benchrealm1") && k8sJobHasCompleted("thumbnail4-benchrealm1")) {
		time.Sleep(500 * time.Millisecond)
	}
	rs0.Append(time.Since(start), 1)
	printResultSummary(rs0)
	evictProcs(mrts.GetRealm(sp.ROOTREALM), ps)
	time.Sleep(10 * time.Second)
	evictMemBlockers(mrts.GetRoot(), blockers)
}
