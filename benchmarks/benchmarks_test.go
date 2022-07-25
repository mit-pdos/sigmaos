package benchmarks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/benchmarks"
	db "ulambda/debug"
	"ulambda/linuxsched"
	"ulambda/proc"
	"ulambda/test"
)

// ========== Common parameters ==========
const (
	OUT_DIR = "name/out_dir"
)

// ========== Nice parameters ==========
const (
	MAT_SIZE        = 2000
	N_TRIALS_NICE   = 10
	CONTENDERS_FRAC = 1.0
)

var MATMUL_NPROCS = linuxsched.NCores
var CONTENDERS_NPROCS = 1

// ========== Micro parameters ==========
const (
	N_TRIALS_MICRO = 1000
	SLEEP_MICRO    = "5000us"
)

// ========== App parameters ==========
const (
	MR_APP                = "mr-grep-wiki2G.yml"
	N_MR_JOBS_APP         = 1
	N_KV_JOBS_APP         = 1
	KV_CLERK_NCLERKS_APP  = 1
	KV_CLERK_DURATION_APP = "90s"
	KV_CLERK_NCORE_APP    = 1
	KV_KVD_NCORE_APP      = 2
)

// ========== Realm parameters ==========
const (
	N_TRIALS_REALM       = 1000
	BALANCE_REALM_1      = "arielck"
	BALANCE_REALM_2      = "test-realm"
	BALANCE_MR_APP_REALM = "mr-grep-wiki2G.yml"
)

var TOTAL_N_CORES_SIGMA_REALM = 0

// Length of time required to do a simple matrix multiplication.
func TestNiceMatMulBaseline(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_NICE)
	_, ps := makeNProcs(N_TRIALS_NICE, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 1)
	runOps(ts, ps, runProc, rs)
	printResults(rs)
	ts.Shutdown()
}

// Start a bunch of spinning procs to contend with one matmul task, and then
// see how long the matmul task took.
func TestNiceMatMulWithSpinners(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_NICE)
	makeOutDir(ts)
	nContenders := int(float64(linuxsched.NCores) / CONTENDERS_FRAC)
	// Make some spinning procs to take up nContenders cores.
	psSpin, _ := makeNProcs(nContenders, "user/spinner", []string{OUT_DIR}, []string{fmt.Sprintf("GOMAXPROCS=%v", CONTENDERS_NPROCS)}, 0)
	// Burst-spawn BE procs
	spawnBurstProcs(ts, psSpin)
	// Wait for the procs to start
	waitStartProcs(ts, psSpin)
	// Make the LC proc.
	_, ps := makeNProcs(N_TRIALS_NICE, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 1)
	// Spawn the LC procs
	runOps(ts, ps, runProc, rs)
	printResults(rs)
	evictProcs(ts, psSpin)
	rmOutDir(ts)
	ts.Shutdown()
}

// Invert the nice relationship. Make spinners high-priority, and make matul
// low priority. This is intended to verify that changing priorities does
// actually affect application throughput for procs which have their priority
// lowered, and by how much.
func TestNiceMatMulWithSpinnersLCNiced(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_NICE)
	makeOutDir(ts)
	nContenders := int(float64(linuxsched.NCores) / CONTENDERS_FRAC)
	// Make some spinning procs to take up nContenders cores. (AS LC)
	psSpin, _ := makeNProcs(nContenders, "user/spinner", []string{OUT_DIR}, []string{fmt.Sprintf("GOMAXPROCS=%v", CONTENDERS_NPROCS)}, 1)
	// Burst-spawn spinning procs
	spawnBurstProcs(ts, psSpin)
	// Wait for the procs to start
	waitStartProcs(ts, psSpin)
	// Make the matmul procs.
	_, ps := makeNProcs(N_TRIALS_NICE, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 0)
	// Spawn the matmul procs
	runOps(ts, ps, runProc, rs)
	printResults(rs)
	evictProcs(ts, psSpin)
	rmOutDir(ts)
	ts.Shutdown()
}

// Test how long it takes to init a semaphore.
func TestMicroInitSemaphore(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_MICRO)
	makeOutDir(ts)
	_, is := makeNSemaphores(ts, N_TRIALS_MICRO)
	runOps(ts, is, initSemaphore, rs)
	printResults(rs)
	rmOutDir(ts)
	ts.Shutdown()
}

// Test how long it takes to up a semaphore.
func TestMicroUpSemaphore(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_MICRO)
	makeOutDir(ts)
	_, is := makeNSemaphores(ts, N_TRIALS_MICRO)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(ts, time.Now(), i)
	}
	runOps(ts, is, upSemaphore, rs)
	printResults(rs)
	rmOutDir(ts)
	ts.Shutdown()
}

// Test how long it takes to down a semaphore.
func TestMicroDownSemaphore(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_MICRO)
	makeOutDir(ts)
	_, is := makeNSemaphores(ts, N_TRIALS_MICRO)
	// Init semaphores first.
	for _, i := range is {
		initSemaphore(ts, time.Now(), i)
		upSemaphore(ts, time.Now(), i)
	}
	runOps(ts, is, downSemaphore, rs)
	printResults(rs)
	rmOutDir(ts)
	ts.Shutdown()
}

// Test how long it takes to Spawn, run, and WaitExit a 5ms proc.
func TestMicroSpawnWaitExit5msSleeper(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_MICRO)
	makeOutDir(ts)
	_, ps := makeNProcs(N_TRIALS_MICRO, "user/sleeper", []string{SLEEP_MICRO, OUT_DIR}, []string{}, 1)
	runOps(ts, ps, runProc, rs)
	printResults(rs)
	rmOutDir(ts)
	ts.Shutdown()
}

func TestAppRunMRWC(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_MR_JOBS_APP)
	jobs, apps := makeNMRJobs(ts, N_MR_JOBS_APP, MR_APP)
	// XXX Clean this up/hide this somehow.
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	runOps(ts, apps, runMR, rs)
	printResults(rs)
	ts.Shutdown()
}

func TestAppRunKVRepl(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_KV_JOBS_APP)
	setNCoresSigmaRealm(ts)
	nclerks := []int{0, int(TOTAL_N_CORES_SIGMA_REALM) / 4, int(TOTAL_N_CORES_SIGMA_REALM) / 2, int(TOTAL_N_CORES_SIGMA_REALM) / 4, 0}
	phases := parseDurations(ts, []string{"5s", "5s", "5s", "5s", "5s"})
	jobs, ji := makeNKVJobs(ts, N_KV_JOBS_APP, int(TOTAL_N_CORES_SIGMA_REALM)/6, 3, nclerks, phases, "", 0, 0)
	// XXX Clean this up/hide this somehow.
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	runOps(ts, ji, runKV, rs)
	printResults(rs)
	ts.Shutdown()
}

func TestAppRunKVPerKVDThroughput(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_KV_JOBS_APP)
	setNCoresSigmaRealm(ts)
	nclerks := []int{KV_CLERK_NCLERKS_APP}
	db.DPrintf(db.ALWAYS, "Running with %v clerks", KV_CLERK_NCLERKS_APP)
	jobs, ji := makeNKVJobs(ts, N_KV_JOBS_APP, 1, 0, nclerks, nil, KV_CLERK_DURATION_APP, proc.Tcore(KV_KVD_NCORE_APP), proc.Tcore(KV_CLERK_NCORE_APP))
	// XXX Clean this up/hide this somehow.
	go func() {
		for _, j := range jobs {
			// Wait until ready
			<-j.ready
			// Ack to allow the job to proceed.
			j.ready <- true
		}
	}()
	runOps(ts, ji, runKV, rs)
	printResults(rs)
	ts.Shutdown()
}

// Burst a bunch of spinning procs, and see how long it takes for all of them
// to start.
//
// XXX Maybe we should do a version with procs that don't spin & consume so
// much CPU?
//
// XXX A bit wonky, since we'll want to dealloc all the machines from the
// realms between runs.
//
// XXX We should probably try this one both warm and cold.
func TestRealmSpawnBurstWaitStartSpinners(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(1)
	makeOutDir(ts)
	// Find the total number of cores available for spinners across all machines.
	// We need to get this in order to find out how many spinners to start.
	setNCoresSigmaRealm(ts)
	ps, _ := makeNProcs(TOTAL_N_CORES_SIGMA_REALM, "user/spinner", []string{OUT_DIR}, []string{}, 1)
	runOps(ts, []interface{}{ps}, spawnBurstWaitStartProcs, rs)
	printResults(rs)
	evictProcs(ts, ps)
	rmOutDir(ts)
	ts.Shutdown()
}

// Start a realm with a long-running BE mr job. Then, start a realm with a kv
// job. In phases, ramp the kv job's CPU utilization up and down, and watch the
// realm-level software balance resource requests across realms.
func TestRealmBalance(t *testing.T) {
	done := make(chan bool)
	// Structures for mr
	ts1 := test.MakeTstateRealm(t, BALANCE_REALM_1)
	rs1 := benchmarks.MakeRawResults(1)
	// Structure for kv
	ts2 := test.MakeTstateRealm(t, BALANCE_REALM_2)
	rs2 := benchmarks.MakeRawResults(1)
	// Find the total number of cores available for spinners across all machines.
	ts := test.MakeTstateAll(t)
	setNCoresSigmaRealm(ts)
	// Prep MR job
	mrjobs, mrapps := makeNMRJobs(ts1, 1, BALANCE_MR_APP_REALM)
	// Need at least one kv realm group.
	assert.True(ts2.T, TOTAL_N_CORES_SIGMA_REALM >= 6, "Too few cores to run benchmark: %v < %v", TOTAL_N_CORES_SIGMA_REALM, 6)
	// Prep KV job
	nclerks := []int{0, int(TOTAL_N_CORES_SIGMA_REALM) / 4, int(TOTAL_N_CORES_SIGMA_REALM) / 2, int(TOTAL_N_CORES_SIGMA_REALM) / 4, 0}
	phases := parseDurations(ts2, []string{"5s", "5s", "5s", "5s", "5s"})
	kvjobs, ji := makeNKVJobs(ts2, 1, int(TOTAL_N_CORES_SIGMA_REALM)/6, 0, nclerks, phases, "", 0, 0)
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
	time.Sleep(5 * time.Second)
	// Kick off KV jobs
	kvjobs[0].ready <- true
	// Wait for both jobs to finish.
	<-done
	<-done
	printResults(rs1)
	printResults(rs2)
	ts1.Shutdown()
	ts2.Shutdown()
}
