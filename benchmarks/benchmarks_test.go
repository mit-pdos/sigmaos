package benchmarks_test

import (
	"fmt"
	"math"
	"runtime"
	"strings"
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

type testOp func(*test.Tstate, interface{})

func makeNProcs(n int, prog string, args []string, env []string, ncore proc.Tcore) ([]*proc.Proc, []interface{}) {
	is := make([]interface{}, 0, n)
	ps := make([]*proc.Proc, 0, n)
	for i := 0; i < n; i++ {
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProc(prog, args)
		p.Env = append(p.Env, env...)
		if ncore > 0 {
			p.Type = proc.T_LC
			p.Ncore = ncore
		} else {
			p.Type = proc.T_BE
		}
		ps = append(ps, p)
		is = append(is, p)
	}
	return ps, is
}

func spawnBurstProcs(ts *test.Tstate, ps []*proc.Proc) {
	db.DPrintf("TEST", "Burst-spawning %v procs", len(ps))
	_, errs := ts.SpawnBurst(ps)
	assert.Equal(ts.T, len(errs), 0, "Errors SpawnBurst: %v", errs)
	for _, p := range ps {
		err := ts.WaitStart(p.Pid)
		assert.Nil(ts.T, err, "WaitStart: %v", err)
	}
	db.DPrintf("TEST", "%v burst-spawned procs have all started:", len(ps))
}

// TODO for matmul, possibly only benchmark internal time
func runProc(ts *test.Tstate, x interface{}) {
	p := x.(*proc.Proc)
	err1 := ts.Spawn(p)
	db.DPrintf("TEST1", "Spawned %v", p)
	status, err2 := ts.WaitExit(p.Pid)
	assert.Nil(ts.T, err1, "Failed to Spawn %v", err1)
	assert.Nil(ts.T, err2, "Failed to WaitExit %v", err2)
	// Correctness checks
	assert.True(ts.T, status.IsStatusOK(), "Bad status: %v", status)
}

func evictProcs(ts *test.Tstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.Evict(p.Pid)
		assert.Nil(ts.T, err, "Evict: %v", err)
		status, err := ts.WaitExit(p.Pid)
		assert.True(ts.T, status.IsStatusEvicted(), "Bad status evict: %v", status)
	}
}

func runOps(ts *test.Tstate, is []interface{}, op testOp, rs *benchmarks.RawResults) {
	for i := 0; i < len(is); i++ {
		// Pefrormance vars
		nRPC := ts.ReadSeqNo()
		start := time.Now()

		// Ops we are benchmarking
		op(ts, is[i])

		// Optional counter
		if i%100 == 0 {
			db.DPrintf("TEST", "i = %v", i)
		}

		// Performance bookeeping
		elapsed := float64(time.Since(start).Microseconds())
		nRPC = ts.ReadSeqNo() - nRPC
		db.DPrintf("TEST2", "Latency: %vus", elapsed)
		throughput := float64(1.0) / elapsed
		rs.Data[i].Set(throughput, elapsed, nRPC)
	}
}

func printResults(rs *benchmarks.RawResults) {
	mean := rs.Mean().Latency
	std := rs.StandardDeviation().Latency
	// Round to 2 decimal points.
	ratio := math.Round((std/mean*100.0)*100.0) / 100.0
	// Get info for the caller.
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		db.DFatalf("Couldn't get caller name")
	}
	fnDetails := runtime.FuncForPC(pc)
	n := fnDetails.Name()
	fnName := n[strings.Index(n, ".")+1:]
	db.DPrintf(db.ALWAYS, "\n\nResults: %v\n=====\nLatency\n-----\nMean: %v (usec) Std: %v (usec)\nStd is %v%% of the mean\n=====\n\n", fnName, mean, std, ratio)
}

func makeOutDir(ts *test.Tstate) {
	err := ts.MkDir(OUT_DIR, 0777)
	assert.Nil(ts.T, err, "Couldn't make out dir: %v", err)
}

func rmOutDir(ts *test.Tstate) {
	err := ts.RmDir(OUT_DIR)
	assert.Nil(ts.T, err, "Couldn't rm out dir: %v", err)
}

// Length of time required to do a simple matrix multiplication.
func TestNiceMatMulBaseline(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_NICE)
	_, is := makeNProcs(N_TRIALS_NICE, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 1)
	runOps(ts, is, runProc, rs)
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
	// Make the LC proc.
	_, is := makeNProcs(N_TRIALS_NICE, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 1)
	// Spawn the LC procs
	runOps(ts, is, runProc, rs)
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
	// Make the matmul procs.
	_, is := makeNProcs(N_TRIALS_NICE, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 0)
	// Spawn the matmul procs
	runOps(ts, is, runProc, rs)
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
	_, is := makeNProcs(N_TRIALS_MICRO, "user/sleeper", []string{SLEEP_MICRO, OUT_DIR}, []string{}, 1)
	runOps(ts, is, runProc, rs)
	printResults(rs)
	rmOutDir(ts)
	ts.Shutdown()
}

// Test how long it takes to Spawn, run, and WaitExit a 5ms proc.
func TestMicroSpawnWaitExit5msSleeper(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS_MICRO)
	makeOutDir(ts)
	_, is := makeNProcs(N_TRIALS_MICRO, "user/sleeper", []string{SLEEP_MICRO, OUT_DIR}, []string{}, 1)
	runOps(ts, is, runProc, rs)
	printResults(rs)
	rmOutDir(ts)
	ts.Shutdown()
}
