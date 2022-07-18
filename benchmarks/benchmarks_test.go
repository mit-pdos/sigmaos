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
	SPIN_DIR       = "name/spinners"
	MAT_SIZE       = 2000
	N_TRIALS       = 100
	CONTENDERS_DEN = 0.5
)

func makeNProcs(n int, prog string, args []string, nproc proc.Tcore) []*proc.Proc {
	ps := []*proc.Proc{}
	for i := 0; i < n; i++ {
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProc(prog, args)
		if nproc > 0 {
			p.Type = proc.T_LC
			p.Ncore = nproc
		} else {
			p.Type = proc.T_BE
		}
		ps = append(ps, p)
	}
	return ps
}

func spawnAndWait(ts *test.Tstate, ps []*proc.Proc, rs *benchmarks.RawResults) {
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

// Length of time required to do a simple matrix multiplication.
func TestMatMulBaseline(t *testing.T) {
	ts := test.MakeTstateAll(t)

	rs := benchmarks.MakeRawResults(N_TRIALS)

	ps := makeNProcs(N_TRIALS, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, 1)

	spawnAndWait(ts, ps, rs)

	mean := rs.Mean().Latency
	std := rs.StandardDeviation().Latency
	// Round to 2 decimal points.
	ratio := math.Round((std/mean*100.0)*100.0) / 100.0

	db.DPrintf(db.ALWAYS, "\n\n=====\nLatency\n-----\nMean: %v (usec) Std: %v (sec)\nStd is %v%% of the mean\n=====\n\n", mean, std, ratio)

	ts.Shutdown()
}

// Doesn't change much unless there is contention in the system, so this
// baseline isn't very interesting.
//
//// Length of time required to do a simple matrix multiplication when its
//// priority is lowered.
//func TestMatMulBaselineNiced(t *testing.T) {
//	ts := test.MakeTstateAll(t)
//
//	rs := benchmarks.MakeRawResults(N_TRIALS)
//
//	ps := makeNProcs(N_TRIALS, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, 0)
//
//	spawnAndWait(ts, ps, rs)
//
//	mean := rs.Mean().Latency
//	std := rs.StandardDeviation().Latency
//	// Round to 2 decimal points.
//	ratio := math.Round((std/mean*100.0)*100.0) / 100.0
//
//	db.DPrintf(db.ALWAYS, "\n\n=====\nLatency\n-----\nMean: %v (usec) Std: %v (sec)\nStd is %v%% of the mean\n=====\n\n", mean, std, ratio)
//
//	ts.Shutdown()
//}

func TestMatMulWithSpinners(t *testing.T) {
	ts := test.MakeTstateAll(t)

	nContenders := int(float64(linuxsched.NCores) / CONTENDERS_DEN)

	rs := benchmarks.MakeRawResults(N_TRIALS)

	err := ts.MkDir(SPIN_DIR, 0777)
	assert.Nil(ts.T, err, "Couldn't make spinners dir: %v", err)

	// Make some spinning procs to take up nContenders cores.
	psSpin := makeNProcs(nContenders, "user/spinner", []string{SPIN_DIR}, 0)

	// Burst spawn the spinners.
	db.DPrintf("TEST", "Spawning %v spinning BE procs", nContenders)
	_, errs := ts.SpawnBurst(psSpin)
	assert.Equal(ts.T, len(errs), 0, "Errors SpawnBurst: %v", errs)

	for _, p := range psSpin {
		err := ts.WaitStart(p.Pid)
		assert.Nil(ts.T, err, "WaitStart: %v", err)
	}

	db.DPrintf("TEST", "Spawning %v BE procs have all started", nContenders)

	// Make the LC proc.
	ps := makeNProcs(N_TRIALS, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, 1)

	// Spawn the LC procs
	spawnAndWait(ts, ps, rs)

	mean := rs.Mean().Latency
	std := rs.StandardDeviation().Latency
	// Round to 2 decimal points.
	ratio := math.Round((std/mean*100.0)*100.0) / 100.0

	db.DPrintf(db.ALWAYS, "\n\n=====\nLatency\n-----\nMean: %v (usec) Std: %v (sec)\nStd is %v%% of the mean\n=====\n\n", mean, std, ratio)

	for _, p := range psSpin {
		err := ts.Evict(p.Pid)
		assert.Nil(ts.T, err, "Evict: %v", err)
		status, err := ts.WaitExit(p.Pid)
		assert.True(ts.T, status.IsStatusEvicted(), "Bad status evict: %v", status)
	}

	ts.Shutdown()
}

func TestMatMulWithSpinnersNiced(t *testing.T) {
	ts := test.MakeTstateAll(t)

	nContenders := int(float64(linuxsched.NCores) / CONTENDERS_DEN)

	rs := benchmarks.MakeRawResults(N_TRIALS)

	err := ts.MkDir(SPIN_DIR, 0777)
	assert.Nil(ts.T, err, "Couldn't make spinners dir: %v", err)

	// Make some spinning procs to take up nContenders cores. (AS LC)
	psSpin := makeNProcs(nContenders, "user/spinner", []string{SPIN_DIR}, 1)

	// Burst spawn the spinners.
	db.DPrintf("TEST", "Spawning %v spinning BE procs", nContenders)
	_, errs := ts.SpawnBurst(psSpin)
	assert.Equal(ts.T, len(errs), 0, "Errors SpawnBurst: %v", errs)

	for _, p := range psSpin {
		err := ts.WaitStart(p.Pid)
		assert.Nil(ts.T, err, "WaitStart: %v", err)
	}

	db.DPrintf("TEST", "Spawning %v BE procs have all started", nContenders)

	// Make the matmul procs.
	ps := makeNProcs(N_TRIALS, "user/matmul", []string{fmt.Sprintf("%v", MAT_SIZE)}, 0)

	// Spawn the matmul procs
	spawnAndWait(ts, ps, rs)

	mean := rs.Mean().Latency
	std := rs.StandardDeviation().Latency
	// Round to 2 decimal points.
	ratio := math.Round((std/mean*100.0)*100.0) / 100.0

	db.DPrintf(db.ALWAYS, "\n\n=====\nLatency\n-----\nMean: %v (usec) Std: %v (sec)\nStd is %v%% of the mean\n=====\n\n", mean, std, ratio)

	for _, p := range psSpin {
		err := ts.Evict(p.Pid)
		assert.Nil(ts.T, err, "Evict: %v", err)
		status, err := ts.WaitExit(p.Pid)
		assert.True(ts.T, status.IsStatusEvicted(), "Bad status evict: %v", status)
	}

	ts.Shutdown()
}
