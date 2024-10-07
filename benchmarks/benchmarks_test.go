package benchmarks_test

import (
	"flag"
	"math/rand"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stretchr/testify/assert"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/hotel"
	"sigmaos/linuxsched"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/scheddclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
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
var MR_ASYNCRW bool
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
var SCHEDD_DURS string
var SCHEDD_MAX_RPS string
var N_CLNT_REQ int
var KVD_MCPU int
var WWWD_MCPU int
var WWWD_REQ_TYPE string
var WWWD_REQ_DELAY time.Duration
var HOTEL_NCACHE int
var HOTEL_CACHE_MCPU int
var N_HOTEL int
var HOTEL_IMG_SZ_MB int
var HOTEL_CACHE_AUTOSCALE bool
var MANUALLY_SCALE_CACHES bool
var SCALE_CACHE_DELAY time.Duration
var N_CACHES_TO_ADD int
var CACHE_TYPE string
var CACHE_GC bool
var BLOCK_MEM string
var N_REALM int

var MEMCACHED_ADDRS string
var HTTP_URL string
var DURATION time.Duration
var MAX_RPS int
var HOTEL_DURS string
var HOTEL_MAX_RPS string
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
	flag.BoolVar(&MR_ASYNCRW, "mr_asyncrw", true, "Mapers and reducers use asynchronous readers/writers.")
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
	flag.StringVar(&SCHEDD_DURS, "schedd_dur", "10s", "Schedd benchmark load generation duration (comma-separated for multiple phases).")
	flag.StringVar(&SCHEDD_MAX_RPS, "schedd_max_rps", "1000", "Max requests/second for schedd bench (comma-separated for multiple phases).")
	flag.IntVar(&N_CLNT_REQ, "nclnt_req", 1, "Number of request each client news.")
	flag.StringVar(&CLERK_DURATION, "clerk_dur", "90s", "Clerk duration.")
	flag.IntVar(&CLERK_MCPU, "clerk_mcpu", 1000, "Clerk mCPU")
	flag.IntVar(&KVD_MCPU, "kvd_mcpu", 2000, "KVD mCPU")
	flag.IntVar(&WWWD_MCPU, "wwwd_mcpu", 2000, "WWWD mCPU")
	flag.StringVar(&WWWD_REQ_TYPE, "wwwd_req_type", "compute", "WWWD request type [compute, dummy, io].")
	flag.DurationVar(&WWWD_REQ_DELAY, "wwwd_req_delay", 500*time.Millisecond, "Average request delay.")
	flag.DurationVar(&SLEEP, "sleep", 0*time.Millisecond, "Sleep length.")
	flag.IntVar(&HOTEL_NCACHE, "hotel_ncache", 1, "Hotel ncache")
	flag.IntVar(&HOTEL_CACHE_MCPU, "hotel_cache_mcpu", 2000, "Hotel cache mcpu")
	flag.IntVar(&HOTEL_IMG_SZ_MB, "hotel_img_sz_mb", 0, "Hotel image data size in megabytes.")
	flag.IntVar(&N_HOTEL, "nhotel", 80, "Number of hotels in the dataset.")
	flag.BoolVar(&HOTEL_CACHE_AUTOSCALE, "hotel_cache_autoscale", false, "Autoscale hotel cache")
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
	flag.StringVar(&IMG_RESIZE_INPUT_PATH, "imgresize_path", "name/s3/~local/9ps3/img/1.jpg", "Path of img resize input file.")
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
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(ts1)
	_, is := newNSemaphores(ts1, N_TRIALS)
	runOps(ts1, is, initSemaphore, rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

// Test how long it takes to up a semaphore.
func TestMicroUpSemaphore(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(ts1)
	_, is := newNSemaphores(ts1, N_TRIALS)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(ts1, i)
	}
	runOps(ts1, is, upSemaphore, rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

// Test how long it takes to down a semaphore.
func TestMicroDownSemaphore(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(ts1)
	_, is := newNSemaphores(ts1, N_TRIALS)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(ts1, i)
		upSemaphore(ts1, i)
	}
	runOps(ts1, is, downSemaphore, rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

// Test how long it takes to cold Spawn and run the first instruction of
// hello-world proc
func TestMicroSpawnWaitStartRealm(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	if PREWARM_REALM {
		rs0 := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
		ps, is := newNProcs(N_TRIALS, "sleeper", []string{"1us", OUT_DIR}, nil, proc.Tmcpu(0))
		runOps(ts1, is, spawnWaitStartProc, rs0)
		waitExitProcs(ts1, ps)
	}

	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(ts1)
	ps, is := newNProcs(N_TRIALS, "spawn-latency", []string{"1us", OUT_DIR}, nil, proc.Tmcpu(0))
	runOps(ts1, is, spawnWaitStartProc, rs)
	waitExitProcs(ts1, ps)
	db.DPrintf(db.BENCH, "Results:\n%v", rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

// Test how long it takes to cold Spawn, run, and WaitExit the rust
// hello-world proc on a min node (i.e., without procq) that hasn't
// run any proc.
func TestMicroSpawnWaitStartNode(t *testing.T) {
	const N = 1
	// const PROG = "sleeper"
	const PROG = "spawn-latency"

	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	sts, err := rootts.GetDir(sp.SCHEDD)
	kernels := sp.Names(sts)
	kernel0 := kernels[0]

	err = rootts.BootMinNode(N)
	assert.Nil(t, err, "Boot node: %v", err)
	db.DPrintf(db.TEST, "Done boot node %d", N)

	sts, err = rootts.GetDir(sp.SCHEDD)
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
		err := ts1.Spawn(p)
		assert.Nil(t, err, "Spawn")
		_, err = ts1.WaitExit(p.GetPid())
		assert.Nil(t, err, "WaitExit")
	}

	time.Sleep(2 * time.Second)

	db.DPrintf(db.TEST, "cold spawn on %v\n", kernel1)
	s := time.Now()
	p := proc.NewProc(PROG, []string{"1us", OUT_DIR})
	p.SetKernels([]string{kernel1})
	err = ts1.Spawn(p)
	assert.Nil(t, err, "Spawn")
	_, err = ts1.WaitExit(p.GetPid())
	assert.Nil(t, err, "WaitExit")
	db.DPrintf(db.BENCH, "Results: %v Cold start %v", kernel1, time.Since(s))

	rootts.Shutdown()
}

// Test how long it takes to Spawn, run, and WaitExit a 5ms proc.
func TestMicroSpawnWaitExit5msSleeper(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if PREWARM_REALM {
		warmupRealm(ts1, []string{"sleeper"})
	}
	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	newOutDir(ts1)
	_, ps := newNProcs(N_TRIALS, "sleeper", []string{"5000us", OUT_DIR}, nil, 1)
	runOps(ts1, ps, runProc, rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

// Test the throughput of spawning procs.
func TestMicroSpawnBurstTpt(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	db.DPrintf(db.ALWAYS, "SpawnBursting %v procs (ncore=%v) with max parallelism %v", N_PROC, MCPU, MAX_PARALLEL)
	ps, _ := newNProcs(N_PROC, "sleeper", []string{"0s", ""}, nil, proc.Tmcpu(MCPU))
	runOps(ts1, []interface{}{ps}, spawnBurstWaitStartProcs, rs)
	printResultSummary(rs)
	waitExitProcs(ts1, ps)
	rootts.Shutdown()
}

func TestMicroWarmupRealm(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.OPS)
	runOps(ts1, []interface{}{"spawn-latency"}, warmupRealmBench, rs)
	printResultSummary(rs)
	rootts.Shutdown()
}

func TestMicroScheddSpawn(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.OPS)

	db.DPrintf(db.BENCH, "rust %v ux %v dummy %v lc %v prewarm %v kpref %v nclnt %v durs %v rps %v skipstats %v", USE_RUST_PROC, DOWNLOAD_FROM_UX, USE_DUMMY_PROC, SPAWN_BENCH_LC_PROC, PREWARM_REALM, WITH_KERNEL_PREF, N_CLNT, SCHEDD_DURS, SCHEDD_MAX_RPS, SKIPSTATS)

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

	sts, err := rootts.GetDir(sp.SCHEDD)
	assert.Nil(rootts.T, err, "Err GetDir schedd: %v", err)
	kernels := sp.Names(sts)
	db.DPrintf(db.TEST, "Kernels %v", kernels)

	// XXX clean up
	// Prep the sleeper bin cache
	db.DPrintf(db.TEST, "Warm up sleeper bin cache on kernel %v", kernels[0])
	p1 := proc.NewProc("sleeper", []string{"1ms", "name/"})
	p1.SetKernels([]string{kernels[0]})
	p1s := []*proc.Proc{p1}
	spawnProcs(ts1, p1s)
	waitStartProcs(ts1, p1s)
	waitExitProcs(ts1, p1s)
	if PREWARM_REALM {
		db.DPrintf(db.TEST, "Warm up remainder of the realm for sleeper")
		warmupRealm(ts1, []string{"sleeper"})
	}
	if !USE_DUMMY_PROC {
		// Cold-start the first target proc to download the bin from S3
		db.DPrintf(db.TEST, "Warm up %v bin cache on kernel %v", prog, kernels[0])
		p2 := proc.NewProc(prog, nil)
		p2.SetKernels([]string{kernels[0]})
		p2s := []*proc.Proc{p2}
		spawnProcs(ts1, p2s)
		waitStartProcs(ts1, p2s)
		if PREWARM_REALM {
			warmupRealm(ts1, []string{prog})
		}
	}

	// Allow the uprocd pool to refill
	time.Sleep(5 * time.Second)

	done := make(chan bool)
	// Prep Schedd job
	scheddJobs, ji := newScheddJobs(ts1, N_CLNT, SCHEDD_DURS, SCHEDD_MAX_RPS, prog, func(sc *sigmaclnt.SigmaClnt, pid sp.Tpid, kernelpref []string) time.Duration {
		if USE_RUST_PROC {
			return runRustSpawnBenchProc(ts1, sc, prog, pid, kernelpref)
		} else if USE_DUMMY_PROC {
			return runDummySpawnBenchProc(ts1, sc, pid, SPAWN_BENCH_LC_PROC)
		} else {
			return runSpawnBenchProc(ts1, sc, pid, kernelpref)
		}
	}, kernels, WITH_KERNEL_PREF, SKIPSTATS)
	// Run Schedd job
	go func() {
		runOps(ts1, ji, runSchedd, rs)
		done <- true
	}()
	// Wait for schedd jobs to set up.
	<-scheddJobs[0].ready
	db.DPrintf(db.TEST, "Schedd setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	// Sleep for a bit
	time.Sleep(SLEEP)
	// Kick off schedd jobs
	scheddJobs[0].ready <- true
	<-done
	db.DPrintf(db.TEST, "Schedd load bench done.")

	printResultSummary(rs)
	rootts.Shutdown()
}

// Test the throughput of spawning procs.
func TestMicroHTTPLoadGen(t *testing.T) {
	RunHTTPLoadGen(HTTP_URL, DURATION, MAX_RPS)
}

func TestAppMR(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if PREWARM_REALM {
		warmupRealm(ts1, []string{"mr-coord", "mr-m-grep", "mr-r-grep", "mr-m-wc", "mr-r-wc"})
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(ts1)
	defer p.Done()
	jobs, apps := newNMRJobs(ts1, p, 1, MR_APP, proc.Tmem(MR_MEM_REQ), MR_ASYNCRW)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	monitorCPUUtil(ts1, p)
	runOps(ts1, apps, runMR, rs)
	printResultSummary(rs)
	rootts.Shutdown()
}

func runKVTest(t *testing.T, nReplicas int) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(ts1)
	defer p.Done()
	nclerks := []int{N_CLERK}
	db.DPrintf(db.ALWAYS, "Running with %v clerks", N_CLERK)
	jobs, ji := newNKVJobs(ts1, 1, N_KVD, nReplicas, nclerks, nil, CLERK_DURATION, proc.Tmcpu(KVD_MCPU), proc.Tmcpu(CLERK_MCPU), KV_AUTO, REDIS_ADDR)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	monitorCPUUtil(ts1, p)
	db.DPrintf(db.TEST, "runOps")
	runOps(ts1, ji, runKV, rs)
	printResultSummary(rs)
	rootts.Shutdown()
}

func TestAppKVUnrepl(t *testing.T) {
	runKVTest(t, 0)
}

func TestAppKVRepl(t *testing.T) {
	runKVTest(t, 3)
}

func TestAppCached(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(ts1)
	defer p.Done()
	const NKEYS = 100
	db.DPrintf(db.ALWAYS, "Running with %v clerks", N_CLERK)
	jobs, ji := newNCachedJobs(ts1, 1, NKEYS, N_KVD, N_CLERK, CLERK_DURATION, proc.Tmcpu(CLERK_MCPU), proc.Tmcpu(KVD_MCPU))
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	monitorCPUUtil(ts1, p)
	runOps(ts1, ji, runCached, rs)
	printResultSummary(rs)
	rootts.Shutdown()
}

// Burst a bunch of spinning procs, and see how long it takes for all of them
// to start.
func TestRealmBurst(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ncores := countClusterCores(rootts) - 1
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	newOutDir(ts1)
	// Find the total number of cores available for spinners across all machines.
	// We need to get this in order to find out how many spinners to start.
	db.DPrintf(db.ALWAYS, "Bursting %v spinning procs", ncores)
	ps, _ := newNProcs(int(ncores), "spinner", []string{OUT_DIR}, nil, 1)
	p := newRealmPerf(ts1)
	defer p.Done()
	monitorCPUUtil(ts1, p)
	runOps(ts1, []interface{}{ps}, spawnBurstWaitStartProcs, rs)
	printResultSummary(rs)
	evictProcs(ts1, ps)
	rmOutDir(ts1)
	rootts.Shutdown()
}

func TestLambdaBurst(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	newOutDir(ts1)
	N_LAMBDAS := 720
	db.DPrintf(db.ALWAYS, "Invoking %v lambdas", N_LAMBDAS)
	ss, is := newNSemaphores(ts1, N_LAMBDAS)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(ts1, i)
	}
	runOps(ts1, []interface{}{ss}, invokeWaitStartLambdas, rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

func TestLambdaInvokeWaitStart(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(720, benchmarks.E2E)
	newOutDir(ts1)
	N_LAMBDAS := 640
	db.DPrintf(db.ALWAYS, "Invoking %v lambdas", N_LAMBDAS)
	_, is := newNSemaphores(ts1, N_LAMBDAS)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(ts1, i)
	}
	runOps(ts1, is, invokeWaitStartOneLambda, rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceMRHotel(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	blockers := blockMem(rootts, BLOCK_MEM)
	// Structures for mr
	ts1, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	// Structure for hotel
	ts2, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(ts2)
	defer p2.Done()
	// Prep MR job
	mrjobs, mrapps := newNMRJobs(ts1, p1, 1, MR_APP, proc.Tmem(MR_MEM_REQ), MR_ASYNCRW)
	// Prep Hotel job
	hotelJobs, ji := newHotelJobs(ts2, p2, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, func(wc *hotel.WebClnt, r *rand.Rand) {
		//		hotel.RunDSB(ts2.T, 1, wc, r)
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "SearchReq %v", err)
	})
	// Monitor cores assigned to MR.
	monitorCPUUtil(ts1, p1)
	// Monitor cores assigned to Hotel.
	monitorCPUUtil(ts2, p2)
	// Run Hotel job
	go func() {
		runOps(ts2, ji, runHotel, rs2)
		done <- true
	}()
	// Wait for hotel jobs to set up.
	<-hotelJobs[0].ready
	db.DPrintf(db.TEST, "Hotel setup done.")
	// Run MR job
	go func() {
		runOps(ts1, mrapps, runMR, rs1)
		done <- true
	}()
	// Wait for MR jobs to set up.
	<-mrjobs[0].ready
	db.DPrintf(db.TEST, "MR setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	if N_CLNT > 1 {
		// Wait for hotel clients to start up on other machines.
		db.DPrintf(db.ALWAYS, "Leader waiting for %v clnts", N_CLNT)
		waitForClnts(rootts, N_CLNT)
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
	evictMemBlockers(rootts, blockers)
	rootts.Shutdown()
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceHotelRPCImgResize(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	blockers := blockMem(rootts, BLOCK_MEM)
	// Structures for imgresize
	ts1, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	if PREWARM_REALM {
		warmupRealm(ts1, []string{"imgresize", "imgresized"})
	}
	// Structure for hotel
	ts2, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(ts2)
	defer p2.Done()
	// Prep ImgResize job
	imgJobs, imgApps := newImgResizeRPCJob(ts1, p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS_PER_SECOND, IMG_RESIZE_DUR, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
	// Prep Hotel job
	hotelJobs, ji := newHotelJobs(ts2, p2, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, func(wc *hotel.WebClnt, r *rand.Rand) {
		//		hotel.RunDSB(ts2.T, 1, wc, r)
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "SearchReq %v", err)
	})
	// Monitor cores assigned to ImgResize.
	monitorCPUUtil(ts1, p1)
	// Monitor cores assigned to Hotel.
	monitorCPUUtil(ts2, p2)
	// Run Hotel job
	go func() {
		runOps(ts2, ji, runHotel, rs2)
		done <- true
	}()
	// Wait for hotel jobs to set up.
	<-hotelJobs[0].ready
	db.DPrintf(db.TEST, "Hotel setup done.")
	// Run ImgResize job
	go func() {
		runOps(ts1, imgApps, runImgResizeRPC, rs1)
		done <- true
	}()
	// Wait for imgResize jobs to set up.
	<-imgJobs[0].ready
	db.DPrintf(db.TEST, "Imgresize setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	if N_CLNT > 1 {
		// Wait for hotel clients to start up on other machines.
		db.DPrintf(db.ALWAYS, "Leader waiting for %v clnts", N_CLNT)
		waitForClnts(rootts, N_CLNT)
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
	evictMemBlockers(rootts, blockers)
	rootts.Shutdown()
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceHotelImgResize(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	blockers := blockMem(rootts, BLOCK_MEM)
	// Structures for imgresize
	ts1, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	if PREWARM_REALM {
		warmupRealm(ts1, []string{"imgresize", "imgresized"})
	}
	// Structure for hotel
	ts2, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(ts2)
	defer p2.Done()
	// Prep ImgResize job
	imgJobs, imgApps := newImgResizeJob(ts1, p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, 0)
	// Prep Hotel job
	hotelJobs, ji := newHotelJobs(ts2, p2, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, func(wc *hotel.WebClnt, r *rand.Rand) {
		//		hotel.RunDSB(ts2.T, 1, wc, r)
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "SearchReq %v", err)
	})
	// Monitor cores assigned to ImgResize.
	monitorCPUUtil(ts1, p1)
	// Monitor cores assigned to Hotel.
	monitorCPUUtil(ts2, p2)
	// Run Hotel job
	go func() {
		runOps(ts2, ji, runHotel, rs2)
		done <- true
	}()
	// Wait for hotel jobs to set up.
	<-hotelJobs[0].ready
	db.DPrintf(db.TEST, "Hotel setup done.")
	// Run ImgResize job
	go func() {
		runOps(ts1, imgApps, runImgResize, rs1)
		done <- true
	}()
	// Wait for imgResize jobs to set up.
	<-imgJobs[0].ready
	db.DPrintf(db.TEST, "Imgresize setup done.")
	db.DPrintf(db.TEST, "Setup phase done.")
	if N_CLNT > 1 {
		// Wait for hotel clients to start up on other machines.
		db.DPrintf(db.ALWAYS, "Leader waiting for %v clnts", N_CLNT)
		waitForClnts(rootts, N_CLNT)
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
	evictMemBlockers(rootts, blockers)
	rootts.Shutdown()
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceMRMR(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	tses := make([]*test.RealmTstate, N_REALM)
	rses := make([]*benchmarks.Results, N_REALM)
	ps := make([]*perf.Perf, N_REALM)
	mrjobs := make([][]*MRJobInstance, N_REALM)
	mrapps := make([][]interface{}, N_REALM)
	// Create structures for MR jobs.
	for i := range tses {
		tsn, err1 := test.NewRealmTstate(rootts, sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)))
		if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
			return
		}
		tses[i] = tsn
		rses[i] = benchmarks.NewResults(1, benchmarks.E2E)
		ps[i] = newRealmPerf(tses[i])
		defer ps[i].Done()
		mrjob, mrapp := newNMRJobs(tses[i], ps[i], 1, MR_APP, proc.Tmem(MR_MEM_REQ), MR_ASYNCRW)
		mrjobs[i] = mrjob
		mrapps[i] = mrapp
	}
	// Start CPU utilization monitoring.
	for i := range tses {
		monitorCPUUtil(tses[i], ps[i])
	}
	// Initialize MR jobs.
	for i := range tses {
		// Start MR job initialization.
		go func(ts *test.RealmTstate, mrapp []interface{}, rs *benchmarks.Results) {
			runOps(ts, mrapp, runMR, rs)
			done <- true
		}(tses[i], mrapps[i], rses[i])
		// Wait for MR job to set up.
		<-mrjobs[i][0].ready
	}
	// Start jobs running, with a small delay between each job start.
	for i := range tses {
		// Kick off MR jobs.
		mrjobs[i][0].ready <- true
		db.DPrintf(db.TEST, "Start MR job %v", i+1)
		// Sleep for a bit before starting the next job
		time.Sleep(SLEEP)
	}
	// Wait for both jobs to finish.
	for i := range tses {
		<-done
		db.DPrintf(db.TEST, "Done MR job %v", i+1)
	}
	printResultSummary(rses[0])
	rootts.Shutdown()
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceImgResizeRPCImgResizeRPC(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	tses := make([]*test.RealmTstate, N_REALM)
	rses := make([]*benchmarks.Results, N_REALM)
	ps := make([]*perf.Perf, N_REALM)
	imgjobs := make([][]*ImgResizeRPCJobInstance, N_REALM)
	imgapps := make([][]interface{}, N_REALM)
	// Create structures for imgresize jobs.
	for i := range tses {
		tsn, err1 := test.NewRealmTstate(rootts, sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)))
		if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
			return
		}
		tses[i] = tsn
		rses[i] = benchmarks.NewResults(1, benchmarks.E2E)
		ps[i] = newRealmPerf(tses[i])
		defer ps[i].Done()
		imgjob, imgapp := newImgResizeRPCJob(tses[i], ps[i], true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS_PER_SECOND, IMG_RESIZE_DUR, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
		imgjobs[i] = imgjob
		imgapps[i] = imgapp
	}
	// Start CPU utilization monitoring.
	for i := range tses {
		monitorCPUUtil(tses[i], ps[i])
	}
	// Initialize ImgResizeRPC jobs.
	for i := range tses {
		// Start ImgResizeRPCjob initialization.
		go func(ts *test.RealmTstate, imgapp []interface{}, rs *benchmarks.Results) {
			runOps(ts, imgapp, runImgResizeRPC, rs)
			done <- true
		}(tses[i], imgapps[i], rses[i])
		// Wait for ImgResizeRPC job to set up.
		<-imgjobs[i][0].ready
	}
	// Start jobs running, with a small delay between each job start.
	for i := range tses {
		// Kick off ImgResizeRPC jobs.
		imgjobs[i][0].ready <- true
		db.DPrintf(db.TEST, "Start ImgResizeRPC job %v", i+1)
		// Sleep for a bit before starting the next job
		time.Sleep(SLEEP)
	}
	// Wait for both jobs to finish.
	for i := range tses {
		<-done
		db.DPrintf(db.TEST, "Done ImgResizeRPC job %v", i+1)
	}
	printResultSummary(rses[0])
	rootts.Shutdown()
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceImgResizeImgResize(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	tses := make([]*test.RealmTstate, N_REALM)
	rses := make([]*benchmarks.Results, N_REALM)
	ps := make([]*perf.Perf, N_REALM)
	imgjobs := make([][]*ImgResizeJobInstance, N_REALM)
	imgapps := make([][]interface{}, N_REALM)
	// Create structures for imgresize jobs.
	for i := range tses {
		tsn, err1 := test.NewRealmTstate(rootts, sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)))
		if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
			return
		}
		tses[i] = tsn
		rses[i] = benchmarks.NewResults(1, benchmarks.E2E)
		ps[i] = newRealmPerf(tses[i])
		defer ps[i].Done()
		imgjob, imgapp := newImgResizeJob(tses[i], ps[i], true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
		imgjobs[i] = imgjob
		imgapps[i] = imgapp
	}
	// Start CPU utilization monitoring.
	for i := range tses {
		monitorCPUUtil(tses[i], ps[i])
	}
	// Initialize ImgResize jobs.
	for i := range tses {
		// Start ImgResize job initialization.
		go func(ts *test.RealmTstate, imgapp []interface{}, rs *benchmarks.Results) {
			runOps(ts, imgapp, runImgResize, rs)
			done <- true
		}(tses[i], imgapps[i], rses[i])
		// Wait for ImgResize job to set up.
		<-imgjobs[i][0].ready
	}
	// Start jobs running, with a small delay between each job start.
	for i := range tses {
		// Kick off ImgResize jobs.
		imgjobs[i][0].ready <- true
		db.DPrintf(db.TEST, "Start ImgResize job %v", i+1)
		// Sleep for a bit before starting the next job
		time.Sleep(SLEEP)
	}
	// Wait for both jobs to finish.
	for i := range tses {
		<-done
		db.DPrintf(db.TEST, "Done ImgResize job %v", i+1)
	}
	printResultSummary(rses[0])
	rootts.Shutdown()
}

// Old realm balance benchmark involving KV & MR.
// Start a realm with a long-running BE mr job. Then, start a realm with a kv
// job. In phases, ramp the kv job's CPU utilization up and down, and watch the
// realm-level software balance resource requests across realms.
func TestKVMRRRB(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	// Structures for mr
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	// Structure for kv
	ts2, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(ts2)
	defer p2.Done()
	// Prep MR job
	mrjobs, mrapps := newNMRJobs(ts1, p1, 1, MR_APP, proc.Tmem(MR_MEM_REQ), MR_ASYNCRW)
	// Prep KV job
	nclerks := []int{N_CLERK}
	kvjobs, ji := newNKVJobs(ts2, 1, N_KVD, 0, nclerks, nil, CLERK_DURATION, proc.Tmcpu(KVD_MCPU), proc.Tmcpu(CLERK_MCPU), KV_AUTO, REDIS_ADDR)
	monitorCPUUtil(ts1, p1)
	monitorCPUUtil(ts2, p2)
	// Run KV job
	go func() {
		runOps(ts2, ji, runKV, rs2)
		done <- true
	}()
	// Wait for KV jobs to set up.
	<-kvjobs[0].ready
	// Run MR job
	go func() {
		runOps(ts1, mrapps, runMR, rs1)
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
	rootts.Shutdown()
}

func testWww(t *testing.T, sigmaos bool) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	db.DPrintf(db.ALWAYS, "Running with %d clients", N_CLNT)
	jobs, ji := newWwwJobs(ts1, sigmaos, 1, proc.Tmcpu(WWWD_MCPU), WWWD_REQ_TYPE, N_TRIALS, N_CLNT, N_CLNT_REQ, WWWD_REQ_DELAY)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	if sigmaos {
		p := newRealmPerf(ts1)
		defer p.Done()
		monitorCPUUtil(ts1, p)
	}
	runOps(ts1, ji, runWww, rs)
	printResultSummary(rs)
	if sigmaos {
		rootts.Shutdown()
	}
}

func TestWwwSigmaos(t *testing.T) {
	testWww(t, true)
}

func TestWwwK8s(t *testing.T) {
	testWww(t, false)
}

func testHotel(rootts *test.Tstate, ts1 *test.RealmTstate, p *perf.Perf, sigmaos bool, fn hotelFn) {
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	jobs, ji := newHotelJobs(ts1, p, sigmaos, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, fn)
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
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	testSocialNet(rootts, ts1, p1, true)
}

func TestSocialNetK8s(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	testSocialNet(rootts, ts1, p1, false)
}

func TestHotelSigmaosSearch(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	testHotel(rootts, ts1, nil, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "Error search req: %v", err)
	})
}

func TestHotelSigmaosSearchScaleCacheDev(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	N := 3
	err := rootts.BootMinNode(N)
	assert.Nil(t, err, "Boot node: %v", err)
	db.DPrintf(db.TEST, "Done boot node %d", N)
	db.DPrintf(db.TEST, "Done boot node %d", N)
	testHotel(rootts, ts1, nil, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "Error search req: %v", err)
	})
}

func TestHotelSigmaosJustCliSearch(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstateClnt(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	clientReady(rootts)
	// Sleep for a bit
	time.Sleep(SLEEP)
	jobs, ji := newHotelJobsCli(ts1, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, func(wc *hotel.WebClnt, r *rand.Rand) {
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
	runOps(ts1, ji, runHotel, rs)
	//	printResultSummary(rs)
	//	jobs[0].requestK8sStats()
}

func TestHotelK8sJustCliSearch(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstateClnt(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	db.DPrintf(db.ALWAYS, "Clnt ready")
	clientReady(rootts)
	db.DPrintf(db.ALWAYS, "Clnt done waiting")
	jobs, ji := newHotelJobsCli(ts1, false, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, CACHE_TYPE, proc.Tmcpu(HOTEL_CACHE_MCPU), MANUALLY_SCALE_CACHES, SCALE_CACHE_DELAY, N_CACHES_TO_ADD, func(wc *hotel.WebClnt, r *rand.Rand) {
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
	runOps(ts1, ji, runHotel, rs)
	//	printResultSummary(rs)
	//	jobs[0].requestK8sStats()
}

func TestHotelK8sSearch(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	testHotel(rootts, ts1, nil, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		err := hotel.RandSearchReq(wc, r)
		assert.Nil(t, err, "Error search req: %v", err)
	})
	downloadS3Results(rootts, filepath.Join("name/s3/~any/9ps3/", "hotelperf/k8s"), HOSTTMP+"sigmaos-perf")
}

func TestHotelK8sSearchCli(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	testHotel(rootts, ts1, nil, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RandSearchReq(wc, r)
	})
}

func TestHotelSigmaosAll(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	testHotel(rootts, ts1, nil, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RunDSB(rootts.T, 1, wc, r)
	})
}

func TestHotelK8sAll(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	testHotel(rootts, ts1, nil, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RunDSB(rootts.T, 1, wc, r)
	})
}

func TestMRK8s(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	assert.NotEqual(rootts.T, K8S_LEADER_NODE_IP, "", "Must pass k8s leader node ip")
	assert.NotEqual(rootts.T, S3_RES_DIR, "", "Must pass s3 reulst dir")
	if K8S_LEADER_NODE_IP == "" || S3_RES_DIR == "" {
		db.DPrintf(db.ALWAYS, "Skipping mr k8s")
		return
	}
	c := startK8sMR(rootts, k8sMRAddr(K8S_LEADER_NODE_IP, MR_K8S_INIT_PORT))
	waitK8sMR(rootts, c)
	downloadS3Results(rootts, filepath.Join("name/s3/~any/9ps3/", S3_RES_DIR), HOSTTMP+"sigmaos-perf")
}

func TestK8sMRMulti(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	assert.NotEqual(rootts.T, K8S_LEADER_NODE_IP, "", "Must pass k8s leader node ip")
	assert.NotEqual(rootts.T, S3_RES_DIR, "", "Must pass s3 result dir")
	if K8S_LEADER_NODE_IP == "" || S3_RES_DIR == "" {
		db.DPrintf(db.ALWAYS, "Skipping mr k8s")
		return
	}
	// Create realm structures.
	ts := make([]*test.RealmTstate, 0, N_REALM)
	ps := make([]*perf.Perf, 0, N_REALM)
	for i := 0; i < N_REALM; i++ {
		rName := sp.Trealm(REALM_BASENAME.String() + strconv.Itoa(i+1))
		db.DPrintf(db.TEST, "Create realm srtructs for %v", rName)
		tsn, err1 := test.NewRealmTstate(rootts, rName)
		if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
			return
		}
		ts = append(ts, tsn)
		ps = append(ps, newRealmPerf(ts[i]))
		defer ps[i].Done()
	}
	db.DPrintf(db.TEST, "Done creating realm srtructs")
	err := ts[0].MkDir(sp.K8S_SCRAPER, 0777)
	assert.Nil(rootts.T, err, "Error mkdir %v", err)
	// Start up the stat scraper procs.
	sdc := scheddclnt.NewScheddClnt(ts[0].SigmaClnt.FsLib)
	nSchedd, err := sdc.Nschedd()
	ps2, _ := newNProcs(nSchedd, "k8s-stat-scraper", []string{}, nil, proc.Tmcpu(1000*(linuxsched.GetNCores()-1)))
	spawnBurstProcs(ts[0], ps2)
	waitStartProcs(ts[0], ps2)

	cs := make([]*rpc.Client, 0, N_REALM)
	for i := 0; i < N_REALM; i++ {
		rName := sp.Trealm(REALM_BASENAME.String() + strconv.Itoa(i+1))
		db.DPrintf(db.TEST, "Starting MR job for realm %v", rName)
		// Start the next k8s job.
		cs = append(cs, startK8sMR(rootts, k8sMRAddr(K8S_LEADER_NODE_IP, MR_K8S_INIT_PORT+i+1)))
		// Monitor cores assigned to this realm.
		//		monitorK8sCPUUtil(ts[i], ps[i], "mr", rName)
		monitorK8sCPUUtilScraperTS(ts[0], ps[i], "Guaranteed")
		// Sleep for a bit before starting the next job
		time.Sleep(SLEEP)
	}
	db.DPrintf(db.TEST, "Done starting MR jobs")
	for i, c := range cs {
		waitK8sMR(rootts, c)
		db.DPrintf(db.TEST, "MR job %v finished", i)
	}
	db.DPrintf(db.TEST, "Done waiting for MR jobs.")
	for i := 0; i < N_REALM; i++ {
		downloadS3ResultsRealm(
			rootts,
			filepath.Join("name/s3/~any/9ps3/", S3_RES_DIR+"-"+strconv.Itoa(i+1)),
			HOSTTMP+"sigmaos-perf",
			sp.Trealm(REALM_BASENAME.String()+strconv.Itoa(i+1)),
		)
	}
	db.DPrintf(db.TEST, "Done downloading results.")
}

func TestK8sBalanceHotelMR(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	// Structures for mr
	ts1, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	// Structure for hotel
	ts2, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p2 := newRealmPerf(ts2)
	defer p2.Done()
	// Monitor cores assigned to MR.
	monitorK8sCPUUtil(ts1, p1, "mr", "")
	// Monitor cores assigned to Hotel.
	monitorK8sCPUUtil(ts2, p2, "hotel", "")
	assert.NotEqual(rootts.T, K8S_LEADER_NODE_IP, "", "Must pass k8s leader node ip")
	assert.NotEqual(rootts.T, S3_RES_DIR, "", "Must pass k8s leader node ip")
	db.DPrintf(db.TEST, "Starting hotel")
	done := make(chan bool)
	go func() {
		testHotel(rootts, ts2, nil, false, func(wc *hotel.WebClnt, r *rand.Rand) {
			hotel.RandSearchReq(wc, r)
		})
		done <- true
	}()
	db.DPrintf(db.TEST, "Starting mr")
	if K8S_LEADER_NODE_IP == "" || S3_RES_DIR == "" {
		db.DPrintf(db.ALWAYS, "Skipping mr k8s")
		return
	}
	c := startK8sMR(rootts, k8sMRAddr(K8S_LEADER_NODE_IP, MR_K8S_INIT_PORT))
	waitK8sMR(rootts, c)
	<-done
	db.DPrintf(db.TEST, "Downloading results")
	downloadS3Results(rootts, filepath.Join("name/s3/~any/9ps3/", S3_RES_DIR), HOSTTMP+"sigmaos-perf")
	downloadS3Results(rootts, filepath.Join("name/s3/~any/9ps3/", "hotelperf/k8s"), HOSTTMP+"sigmaos-perf")
}

func TestImgResize(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if PREWARM_REALM {
		warmupRealm(ts1, []string{"imgresize", "imgresized"})
	}
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(ts1)
	defer p.Done()
	jobs, apps := newImgResizeJob(ts1, p, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	monitorCPUUtil(ts1, p)
	runOps(ts1, apps, runImgResize, rs)
	printResultSummary(rs)
	rootts.Shutdown()
}

func TestK8sImgResize(t *testing.T) {
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts1, err1 := test.NewRealmTstateClnt(rootts, sp.ROOTREALM)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	if PREWARM_REALM {
		warmupRealm(ts1, nil)
	}
	sdc := scheddclnt.NewScheddClnt(ts1.FsLib)
	nSchedd, err := sdc.Nschedd()
	assert.Nil(ts1.Ts.T, err, "Error nschedd %v", err)
	rs := benchmarks.NewResults(1, benchmarks.E2E)
	p := newRealmPerf(ts1)
	defer p.Done()
	err = ts1.MkDir(sp.K8S_SCRAPER, 0777)
	assert.Nil(ts1.Ts.T, err, "Error mkdir %v", err)
	// Start up the stat scraper procs.
	ps, _ := newNProcs(nSchedd, "k8s-stat-scraper", []string{}, nil, proc.Tmcpu(1000*(linuxsched.GetNCores()-1)))
	spawnBurstProcs(ts1, ps)
	waitStartProcs(ts1, ps)
	// NOte start time
	start := time.Now()
	// Monitor CPU utilization via the stat scraper procs.
	monitorK8sCPUUtilScraper(rootts, p, "BestEffort")
	exec.Command("kubectl", "apply", "-Rf", "/tmp/thumbnail.yaml").Start()
	for !k8sJobHasCompleted(K8S_JOB_NAME) {
		time.Sleep(500 * time.Millisecond)
	}
	rs.Append(time.Since(start), 1)
	printResultSummary(rs)
	evictProcs(ts1, ps)
	rootts.Shutdown()
}

func TestRealmBalanceSimpleImgResize(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	blockers := blockMem(rootts, BLOCK_MEM)
	// Structures for BE image resize
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	// Structure for LC image resize
	ts2, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(ts2)
	defer p2.Done()
	// Prep resize jobs
	imgJobsBE, imgAppsBE := newImgResizeJob(
		ts1, p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, 0, proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))
	imgJobsLC, imgAppsLC := newImgResizeJob(
		ts2, p2, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, proc.Tmcpu(IMG_RESIZE_MCPU), proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, proc.Tmcpu(1000))

	// Run image resize jobs
	go func() {
		runOps(ts1, imgAppsBE, runImgResize, rs1)
		done <- true
	}()
	go func() {
		runOps(ts2, imgAppsLC, runImgResize, rs2)
		done <- true
	}()
	// Wait for image resize jobs to set up.
	<-imgJobsBE[0].ready
	<-imgJobsLC[0].ready

	// Monitor cores for kernel procs
	monitorCPUUtil(ts1, p1)
	monitorCPUUtil(ts2, p2)
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
	evictMemBlockers(rootts, blockers)
	rootts.Shutdown()
}

func TestRealmBalanceSocialNetworkImgResize(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	blockers := blockMem(rootts, BLOCK_MEM)
	ts0, err1 := test.NewRealmTstateClnt(rootts, "rootrealm")
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p0 := newRealmPerf(ts0)
	defer p0.Done()
	// Structures for image resize
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	// Structure for social network
	ts2, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(ts2)
	defer p2.Done()
	// Prep image resize job
	imgJobs, imgApps := newImgResizeJob(
		ts1, p1, true, IMG_RESIZE_INPUT_PATH, N_IMG_RESIZE_TASKS, N_IMG_RESIZE_INPUTS_PER_TASK, 0, proc.Tmem(IMG_RESIZE_MEM_MB), IMG_RESIZE_N_ROUNDS, 0)
	// Prep social network job
	snJobs, snApps := newSocialNetworkJobs(ts2, p2, true, SOCIAL_NETWORK_READ_ONLY, SOCIAL_NETWORK_DURS, SOCIAL_NETWORK_MAX_RPS, 3)
	// Run social network job
	go func() {
		runOps(ts2, snApps, runSocialNetwork, rs2)
		done <- true
	}()
	// Wait for social network jobs to set up.
	<-snJobs[0].ready
	db.DPrintf(db.TEST, "Social Network setup done.")
	// Run image resize job
	go func() {
		runOps(ts1, imgApps, runImgResize, rs1)
		done <- true
	}()
	// Wait for image resize jobs to set up.
	<-imgJobs[0].ready
	// Monitor cores for kernel procs
	monitorCPUUtil(ts0, p0)
	monitorCPUUtil(ts1, p1)
	monitorCPUUtil(ts2, p2)
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
	evictMemBlockers(rootts, blockers)
	rootts.Shutdown()
}

func TestK8sSocialNetworkImgResize(t *testing.T) {
	done := make(chan bool)
	rootts, err1 := test.NewTstateWithRealms(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	blockers := blockMem(rootts, BLOCK_MEM)
	// make realm to run k8s scrapper
	ts0, err1 := test.NewRealmTstateClnt(rootts, sp.ROOTREALM)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p0 := newRealmPerf(ts0)
	defer p0.Done()
	if PREWARM_REALM {
		warmupRealm(ts0, nil)
	}
	sdc := scheddclnt.NewScheddClnt(ts0.SigmaClnt.FsLib)
	nSchedd, err := sdc.Nschedd()
	assert.Nil(ts0.Ts.T, err, "Error nschedd %v", err)
	rs0 := benchmarks.NewResults(1, benchmarks.E2E)
	err = ts0.MkDir(sp.K8S_SCRAPER, 0777)
	assert.Nil(ts0.Ts.T, err, "Error mkdir %v", err)
	// Start up the stat scraper procs.
	//ps, _ := newNProcs(nSchedd, "k8s-stat-scraper", []string{}, nil, proc.Tmcpu(1000*(linuxsched.GetNCores()-1)))
	ps, _ := newNProcs(nSchedd, "k8s-stat-scraper", []string{}, nil, 0)
	spawnBurstProcs(ts0, ps)
	waitStartProcs(ts0, ps)
	// Structures for image resize
	ts1, err1 := test.NewRealmTstate(rootts, REALM1)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	//rs1 := benchmarks.NewResults(1, benchmarks.E2E)
	p1 := newRealmPerf(ts1)
	defer p1.Done()
	// Structure for social network
	ts2, err1 := test.NewRealmTstate(rootts, REALM2)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	rs2 := benchmarks.NewResults(1, benchmarks.E2E)
	p2 := newRealmPerf(ts2)
	defer p2.Done()
	// Prep image resize job

	// Prep social network job
	snJobs, snApps := newSocialNetworkJobs(ts2, p2, false, SOCIAL_NETWORK_READ_ONLY, SOCIAL_NETWORK_DURS, SOCIAL_NETWORK_MAX_RPS, 3)
	// Monitor cores assigned to image resize.
	// NOte start time
	start := time.Now()
	// Run social network job
	go func() {
		runOps(ts2, snApps, runSocialNetwork, rs2)
		snJobs[0].requestK8sStats()
		done <- true
	}()
	// Wait for social network jobs to set up.
	<-snJobs[0].ready
	db.DPrintf(db.TEST, "Social Network setup done.")
	// Monitor CPU utilization via the stat scraper procs.
	monitorK8sCPUUtilScraper(rootts, p2, "Burstable")
	monitorK8sCPUUtilScraper(rootts, p1, "BestEffort")
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
	downloadS3Results(rootts, filepath.Join("name/s3/~any/9ps3/", "social-network-perf/k8s"), HOSTTMP+"sigmaos-perf")
	for !(k8sJobHasCompleted("thumbnail1-benchrealm1") && k8sJobHasCompleted("thumbnail2-benchrealm1") &&
		k8sJobHasCompleted("thumbnail3-benchrealm1") && k8sJobHasCompleted("thumbnail4-benchrealm1")) {
		time.Sleep(500 * time.Millisecond)
	}
	rs0.Append(time.Since(start), 1)
	printResultSummary(rs0)
	evictProcs(ts0, ps)
	time.Sleep(10 * time.Second)
	evictMemBlockers(rootts, blockers)
	rootts.Shutdown()
}
