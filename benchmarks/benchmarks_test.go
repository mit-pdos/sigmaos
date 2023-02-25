package benchmarks_test

import (
	"flag"
	"math/rand"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/hotel"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	REALM1 sp.Trealm = "benchrealm1"
	REALM2           = "benchrealm2"

	HOSTTMP = "/tmp/"
)

// Parameters
var N_TRIALS int
var N_THREADS int
var PREGROW_REALM bool
var MR_APP string
var KV_AUTO string
var N_KVD int
var N_CLERK int
var CLERK_DURATION string
var CLERK_NCORE int
var N_CLNT int
var N_CLNT_REQ int
var KVD_NCORE int
var WWWD_NCORE int
var WWWD_REQ_TYPE string
var WWWD_REQ_DELAY time.Duration
var HOTEL_NCACHE int
var HOTEL_DURS string
var HOTEL_MAX_RPS string
var SLEEP time.Duration
var REDIS_ADDR string
var N_PROC int
var N_CORE int
var MAT_SIZE int
var CONTENDERS_FRAC float64
var GO_MAX_PROCS int
var MAX_PARALLEL int
var K8S_ADDR string
var K8S_LEADER_NODE_IP string
var S3_RES_DIR string

// Read & set the proc version.
func init() {
	flag.IntVar(&N_TRIALS, "ntrials", 1, "Number of trials.")
	flag.IntVar(&N_THREADS, "nthreads", 1, "Number of threads.")
	flag.BoolVar(&PREGROW_REALM, "pregrow_realm", false, "Pre-grow realm to include all cluster resources.")
	flag.StringVar(&MR_APP, "mrapp", "mr-wc-wiki1.8G.yml", "Name of mr yaml file.")
	flag.StringVar(&KV_AUTO, "kvauto", "manual", "KV auto-growing/shrinking.")
	flag.IntVar(&N_KVD, "nkvd", 1, "Number of kvds.")
	flag.IntVar(&N_CLERK, "nclerk", 1, "Number of clerks.")
	flag.IntVar(&N_CLNT, "nclnt", 1, "Number of www clients.")
	flag.IntVar(&N_CLNT_REQ, "nclnt_req", 1, "Number of request each www client makes.")
	flag.StringVar(&CLERK_DURATION, "clerk_dur", "90s", "Clerk duration.")
	flag.IntVar(&CLERK_NCORE, "clerk_ncore", 1, "Clerk Ncore")
	flag.IntVar(&KVD_NCORE, "kvd_ncore", 2, "KVD Ncore")
	flag.IntVar(&WWWD_NCORE, "wwwd_ncore", 2, "WWWD Ncore")
	flag.StringVar(&WWWD_REQ_TYPE, "wwwd_req_type", "compute", "WWWD request type [compute, dummy, io].")
	flag.DurationVar(&WWWD_REQ_DELAY, "wwwd_req_delay", 500*time.Millisecond, "Average request delay.")
	flag.DurationVar(&SLEEP, "sleep", 20*time.Second, "Sleep length.")
	flag.IntVar(&HOTEL_NCACHE, "hotel_ncache", 1, "Hotel ncache")
	flag.StringVar(&HOTEL_DURS, "hotel_dur", "10s", "Hotel benchmark load generation duration (comma-separated for multiple phases).")
	flag.StringVar(&HOTEL_MAX_RPS, "hotel_max_rps", "1000", "Max requests/second for hotel bench (comma-separated for multiple phases).")
	flag.StringVar(&K8S_ADDR, "k8saddr", "", "Kubernetes frontend service address (only for hotel benchmarking for the time being).")
	flag.StringVar(&K8S_LEADER_NODE_IP, "k8sleaderip", "", "Kubernetes leader node ip.")
	flag.StringVar(&S3_RES_DIR, "s3resdir", "", "Results dir in s3.")
	flag.StringVar(&REDIS_ADDR, "redisaddr", "", "Redis server address")
	flag.IntVar(&N_PROC, "nproc", 1, "Number of procs per trial.")
	flag.IntVar(&N_CORE, "ncore", 1, "Generic proc test Ncore")
	flag.IntVar(&MAT_SIZE, "matrixsize", 4000, "Size of matrix.")
	flag.Float64Var(&CONTENDERS_FRAC, "contenders", 4000, "Fraction of cores which should be taken up by contending procs.")
	flag.IntVar(&GO_MAX_PROCS, "gomaxprocs", int(linuxsched.NCores), "Go maxprocs setting for procs to be spawned.")
	flag.IntVar(&MAX_PARALLEL, "max_parallel", 1, "Max amount of parallelism.")
}

// ========== Common parameters ==========
const (
	OUT_DIR = "name/out_dir"
)

// Test how long it takes to init a semaphore.
func TestMicroInitSemaphore(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(N_TRIALS, benchmarks.OPS)
	makeOutDir(ts1)
	_, is := makeNSemaphores(ts1, N_TRIALS)
	runOps(ts1, is, initSemaphore, rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

// Test how long it takes to up a semaphore.
func TestMicroUpSemaphore(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(N_TRIALS, benchmarks.OPS)
	makeOutDir(ts1)
	_, is := makeNSemaphores(ts1, N_TRIALS)
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
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(N_TRIALS, benchmarks.OPS)
	makeOutDir(ts1)
	_, is := makeNSemaphores(ts1, N_TRIALS)
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

// Test how long it takes to Spawn, run, and WaitExit a 5ms proc.
func TestMicroSpawnWaitExit5msSleeper(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(N_TRIALS, benchmarks.OPS)
	makeOutDir(ts1)
	_, ps := makeNProcs(N_TRIALS, "sleeper", []string{"5000us", OUT_DIR}, nil, 1)
	runOps(ts1, ps, runProc, rs)
	printResultSummary(rs)
	rmOutDir(ts1)
	rootts.Shutdown()
}

// Test the throughput of spawning procs.
func TestMicroSpawnBurstTpt(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(N_TRIALS, benchmarks.OPS)
	db.DPrintf(db.ALWAYS, "SpawnBursting %v procs (ncore=%v) with max parallelism %v", N_PROC, N_CORE, MAX_PARALLEL)
	ps, _ := makeNProcs(N_PROC, "sleeper", []string{"0s", ""}, nil, proc.Tcore(N_CORE))
	runOps(ts1, []interface{}{ps}, spawnBurstWaitStartProcs, rs)
	printResultSummary(rs)
	waitExitProcs(ts1, ps)
	rootts.Shutdown()
}

func TestAppMR(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(1, benchmarks.E2E)
	jobs, apps := makeNMRJobs(ts1, 1, MR_APP)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	p := monitorCoresAssigned(ts1)
	defer p.Done()
	runOps(ts1, apps, runMR, rs)
	printResultSummary(rs)
	rootts.Shutdown()
}

func runKVTest(t *testing.T, nReplicas int) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(1, benchmarks.E2E)
	nclerks := []int{N_CLERK}
	db.DPrintf(db.ALWAYS, "Running with %v clerks", N_CLERK)
	jobs, ji := makeNKVJobs(ts1, 1, N_KVD, nReplicas, nclerks, nil, CLERK_DURATION, proc.Tcore(KVD_NCORE), proc.Tcore(CLERK_NCORE), KV_AUTO, REDIS_ADDR)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	p := monitorCoresAssigned(ts1)
	defer p.Done()
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

// Burst a bunch of spinning procs, and see how long it takes for all of them
// to start.
func TestRealmBurst(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	ncores := countClusterCores(rootts) - 1
	rs := benchmarks.MakeResults(1, benchmarks.E2E)
	makeOutDir(ts1)
	// Find the total number of cores available for spinners across all machines.
	// We need to get this in order to find out how many spinners to start.
	db.DPrintf(db.ALWAYS, "Bursting %v spinning procs", ncores)
	ps, _ := makeNProcs(int(ncores), "spinner", []string{OUT_DIR}, nil, 1)
	p := monitorCoresAssigned(ts1)
	defer p.Done()
	runOps(ts1, []interface{}{ps}, spawnBurstWaitStartProcs, rs)
	printResultSummary(rs)
	evictProcs(ts1, ps)
	rmOutDir(ts1)
	rootts.Shutdown()
}

func TestLambdaBurst(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(1, benchmarks.E2E)
	makeOutDir(ts1)
	// Find the total number of cores available for spinners across all machines.
	// We need to get this in order to find out how many spinners to start.
	N_LAMBDAS := 720
	db.DPrintf(db.ALWAYS, "Invoking %v lambdas", N_LAMBDAS)
	ss, is := makeNSemaphores(ts1, N_LAMBDAS)
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
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(720, benchmarks.E2E)
	makeOutDir(ts1)
	N_LAMBDAS := 640
	db.DPrintf(db.ALWAYS, "Invoking %v lambdas", N_LAMBDAS)
	_, is := makeNSemaphores(ts1, N_LAMBDAS)
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
	rootts := test.MakeTstateWithRealms(t)
	// Structures for mr
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs1 := benchmarks.MakeResults(1, benchmarks.E2E)
	// Structure for hotel
	ts2 := test.MakeRealmTstate(rootts, REALM2)
	rs2 := benchmarks.MakeResults(1, benchmarks.E2E)
	// Prep MR job
	mrjobs, mrapps := makeNMRJobs(ts1, 1, MR_APP)
	// Prep Hotel job
	hotelJobs, ji := makeHotelJobs(ts2, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, func(wc *hotel.WebClnt, r *rand.Rand) {
		//		hotel.RunDSB(ts2.T, 1, wc, r)
		hotel.RandSearchReq(wc, r)
	})
	p1 := monitorCoresAssigned(ts1)
	p2 := monitorCoresAssigned(ts2)
	defer p1.Done()
	defer p2.Done()
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
	_ = rs1
	printResultSummary(rs1)
	rootts.Shutdown()
}

// Start a realm with a long-running BE mr job. Then, start a realm with an LC
// hotel job. In phases, ramp the hotel job's CPU utilization up and down, and
// watch the realm-level software balance resource requests across realms.
func TestRealmBalanceMRMR(t *testing.T) {
	done := make(chan bool)
	rootts := test.MakeTstateWithRealms(t)
	// Structures for mr
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs1 := benchmarks.MakeResults(1, benchmarks.E2E)
	// Structure for kv
	ts2 := test.MakeRealmTstate(rootts, REALM2)
	rs2 := benchmarks.MakeResults(1, benchmarks.E2E)
	// Prep MR job
	mrjobs1, mrapps1 := makeNMRJobs(ts1, 1, MR_APP)
	// Prep MR job
	mrjobs2, mrapps2 := makeNMRJobs(ts2, 1, MR_APP)
	p1 := monitorCoresAssigned(ts1)
	p2 := monitorCoresAssigned(ts2)
	defer p1.Done()
	defer p2.Done()
	// Run MR job
	go func() {
		runOps(ts2, mrapps2, runMR, rs2)
		done <- true
	}()
	// Wait for MR jobs to set up.
	<-mrjobs2[0].ready
	// Run MR job
	go func() {
		runOps(ts1, mrapps1, runMR, rs1)
		done <- true
	}()
	// Wait for MR jobs to set up.
	<-mrjobs1[0].ready
	// Kick off MR jobs.
	mrjobs2[0].ready <- true
	db.DPrintf(db.TEST, "Start MR job 1")
	// Sleep for a bit
	time.Sleep(SLEEP)
	db.DPrintf(db.TEST, "Start MR job 2")
	// Kick off hotel jobs
	mrjobs1[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	db.DPrintf(db.TEST, "Done MR job 1")
	<-done
	db.DPrintf(db.TEST, "Done MR job 2")
	printResultSummary(rs1)
	rootts.Shutdown()
}

// Old realm balance benchmark involving KV & MR.
// Start a realm with a long-running BE mr job. Then, start a realm with a kv
// job. In phases, ramp the kv job's CPU utilization up and down, and watch the
// realm-level software balance resource requests across realms.
func TestKVMRRRB(t *testing.T) {
	done := make(chan bool)
	rootts := test.MakeTstateWithRealms(t)
	// Structures for mr
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs1 := benchmarks.MakeResults(1, benchmarks.E2E)
	// Structure for kv
	ts2 := test.MakeRealmTstate(rootts, REALM2)
	rs2 := benchmarks.MakeResults(1, benchmarks.E2E)
	// Prep MR job
	mrjobs, mrapps := makeNMRJobs(ts1, 1, MR_APP)
	// Prep KV job
	nclerks := []int{N_CLERK}
	kvjobs, ji := makeNKVJobs(ts2, 1, N_KVD, 0, nclerks, nil, CLERK_DURATION, proc.Tcore(KVD_NCORE), proc.Tcore(CLERK_NCORE), KV_AUTO, REDIS_ADDR)
	p1 := monitorCoresAssigned(ts1)
	defer p1.Done()
	p2 := monitorCoresAssigned(ts2)
	defer p2.Done()
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
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(1, benchmarks.E2E)
	db.DPrintf(db.ALWAYS, "Running with %d clients", N_CLNT)
	jobs, ji := makeWwwJobs(ts1, sigmaos, 1, proc.Tcore(WWWD_NCORE), WWWD_REQ_TYPE, N_TRIALS, N_CLNT, N_CLNT_REQ, WWWD_REQ_DELAY)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	if sigmaos {
		p := monitorCoresAssigned(ts1)
		defer p.Done()
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

func testHotel(rootts *test.Tstate, ts1 *test.RealmTstate, sigmaos bool, fn hotelFn) {
	rs := benchmarks.MakeResults(1, benchmarks.E2E)
	jobs, ji := makeHotelJobs(ts1, sigmaos, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, fn)
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	if sigmaos {
		p := monitorCoresAssigned(ts1)
		defer p.Done()
	}
	runOps(ts1, ji, runHotel, rs)
	//	printResultSummary(rs)
	if sigmaos {
		rootts.Shutdown()
	} else {
		jobs[0].requestK8sStats()
	}
}

func TestHotelSigmaosSearch(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	testHotel(rootts, ts1, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RandSearchReq(wc, r)
	})
}

func TestHotelSigmaosJustCliSearch(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(1, benchmarks.E2E)
	jobs, ji := makeHotelJobsCli(ts1, true, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RandSearchReq(wc, r)
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
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	rs := benchmarks.MakeResults(1, benchmarks.E2E)
	jobs, ji := makeHotelJobsCli(ts1, false, HOTEL_DURS, HOTEL_MAX_RPS, HOTEL_NCACHE, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RandSearchReq(wc, r)
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
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	testHotel(rootts, ts1, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RandSearchReq(wc, r)
	})
}

func TestHotelK8sSearchCli(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	testHotel(rootts, ts1, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RandSearchReq(wc, r)
	})
}

func TestHotelSigmaosAll(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	testHotel(rootts, ts1, true, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RunDSB(rootts.T, 1, wc, r)
	})
}

func TestHotelK8sAll(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	testHotel(rootts, ts1, false, func(wc *hotel.WebClnt, r *rand.Rand) {
		hotel.RunDSB(rootts.T, 1, wc, r)
	})
}

func TestMRK8s(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	assert.NotEqual(rootts.T, K8S_LEADER_NODE_IP, "", "Must pass k8s leader node ip")
	assert.NotEqual(rootts.T, S3_RES_DIR, "", "Must pass k8s leader node ip")
	if K8S_LEADER_NODE_IP == "" || S3_RES_DIR == "" {
		db.DPrintf(db.ALWAYS, "Skipping mr k8s")
		return
	}
	c := startK8sMR(rootts, K8S_LEADER_NODE_IP+":32585")
	waitK8sMR(rootts, c)
	downloadS3Results(rootts, path.Join("name/s3/~any/9ps3/", S3_RES_DIR), HOSTTMP+"perf-output")
}

func TestK8sBalanceHotelMR(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)
	assert.NotEqual(rootts.T, K8S_LEADER_NODE_IP, "", "Must pass k8s leader node ip")
	assert.NotEqual(rootts.T, S3_RES_DIR, "", "Must pass k8s leader node ip")
	db.DPrintf(db.TEST, "Starting hotel")
	done := make(chan bool)
	go func() {
		testHotel(rootts, ts1, false, func(wc *hotel.WebClnt, r *rand.Rand) {
			hotel.RandSearchReq(wc, r)
		})
		done <- true
	}()
	db.DPrintf(db.TEST, "Starting mr")
	if K8S_LEADER_NODE_IP == "" || S3_RES_DIR == "" {
		db.DPrintf(db.ALWAYS, "Skipping mr k8s")
		return
	}
	c := startK8sMR(rootts, K8S_LEADER_NODE_IP+":32585")
	waitK8sMR(rootts, c)
	<-done
	db.DPrintf(db.TEST, "Downloading results")
	downloadS3Results(rootts, path.Join("name/s3/~any/9ps3/", S3_RES_DIR), HOSTTMP+"perf-output")
	downloadS3Results(rootts, path.Join("name/s3/~any/9ps3/", "hotelperf/k8s"), HOSTTMP+"perf-output")
}
