package benchmarks_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/benchmarks"
	db "ulambda/debug"
	"ulambda/linuxsched"
	"ulambda/proc"
	"ulambda/test"
)

const (
	SPIN_DIR        = "name/spinners"
	MAT_SIZE        = 2000
	N_TRIALS        = 10
	CONTENDERS_FRAC = 1.0
)

var MATMUL_NPROCS = linuxsched.NCores
var CONTENDERS_NPROCS = 1

func makeNProcs(n int, prog string, args []string, env []string, ncore proc.Tcore) []*proc.Proc {
	ps := []*proc.Proc{}
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
	}
	return ps
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

func evictProcs(ts *test.Tstate, ps []*proc.Proc) {
	for _, p := range ps {
		err := ts.Evict(p.Pid)
		assert.Nil(ts.T, err, "Evict: %v", err)
		status, err := ts.WaitExit(p.Pid)
		assert.True(ts.T, status.IsStatusEvicted(), "Bad status evict: %v", status)
	}
}

func runProcs(ts *test.Tstate, ps []*proc.Proc, rs *benchmarks.RawResults) {
	for i := 0; i < len(ps); i++ {
		err := ts.Spawn(ps[i])
		db.DPrintf("TEST1", "Spawned %v", ps[i])
		assert.Nil(ts.T, err, "Failed to Spawn %v", err)
		status, err := ts.WaitExit(ps[i].Pid)
		assert.Nil(ts.T, err, "Failed to WaitExit %v", err)
		assert.True(ts.T, status.IsStatusOK(), "Bad status: %v", status)

		if i%100 == 0 {
			db.DPrintf("TEST", "i = %v", i)
		}

		elapsed := status.Data().(float64)
		db.DPrintf("TEST2", "Latency: %vus", elapsed)
		throughput := float64(1.0) / elapsed
		rs.Data[i].Set(throughput, elapsed, 0)
	}
}

func printResults(rs *benchmarks.RawResults) {
	mean := rs.Mean().Latency
	std := rs.StandardDeviation().Latency
	// Round to 2 decimal points.
	ratio := math.Round((std/mean*100.0)*100.0) / 100.0
	db.DPrintf(db.ALWAYS, "\n\n=====\nLatency\n-----\nMean: %v (usec) Std: %v (sec)\nStd is %v%% of the mean\n=====\n\n", mean, std, ratio)
}

// Length of time required to do a simple matrix multiplication.
func TestMicroBenchmarkMatMulBaseline(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS)
	ps := makeNProcs(N_TRIALS, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 1)
	runProcs(ts, ps, rs)
	printResults(rs)
	ts.Shutdown()
}

// Start a bunch of spinning procs to contend with one matmul task, and then
// see how long the matmul task took.
func TestMicroBenchmarkMatMulWithSpinners(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS)
	err := ts.MkDir(SPIN_DIR, 0777)
	assert.Nil(ts.T, err, "Couldn't make spinners dir: %v", err)
	nContenders := int(float64(linuxsched.NCores) / CONTENDERS_FRAC)
	// Make some spinning procs to take up nContenders cores.
	psSpin := makeNProcs(nContenders, "user/spinner", []string{SPIN_DIR}, []string{fmt.Sprintf("GOMAXPROCS=%v", CONTENDERS_NPROCS)}, 0)
	// Burst-spawn BE procs
	spawnBurstProcs(ts, psSpin)
	// Make the LC proc.
	ps := makeNProcs(N_TRIALS, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 1)
	// Spawn the LC procs
	runProcs(ts, ps, rs)
	printResults(rs)
	evictProcs(ts, psSpin)
	ts.Shutdown()
}

// Invert the nice relationship. Make spinners high-priority, and make matul
// low priority. This is intended to verify that changing priorities does
// actually affect application throughput for procs which have their priority
// lowered, and by how much.
func TestMicroBenchmarkMatMulWithSpinnersLCNiced(t *testing.T) {
	ts := test.MakeTstateAll(t)
	rs := benchmarks.MakeRawResults(N_TRIALS)
	err := ts.MkDir(SPIN_DIR, 0777)
	assert.Nil(ts.T, err, "Couldn't make spinners dir: %v", err)
	nContenders := int(float64(linuxsched.NCores) / CONTENDERS_FRAC)
	// Make some spinning procs to take up nContenders cores. (AS LC)
	psSpin := makeNProcs(nContenders, "user/spinner", []string{SPIN_DIR}, []string{fmt.Sprintf("GOMAXPROCS=%v", CONTENDERS_NPROCS)}, 1)
	// Burst-spawn spinning procs
	spawnBurstProcs(ts, psSpin)
	// Make the matmul procs.
	ps := makeNProcs(N_TRIALS, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, []string{fmt.Sprintf("GOMAXPROCS=%v", MATMUL_NPROCS)}, 0)
	// Spawn the matmul procs
	runProcs(ts, ps, rs)
	printResults(rs)
	evictProcs(ts, psSpin)
	ts.Shutdown()
}
