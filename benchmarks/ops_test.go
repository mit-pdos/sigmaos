package benchmarks_test

import (
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/proc"
	"ulambda/semclnt"
	"ulambda/test"
)

//
// The set of basic operations that we benchmark.
//

type testOp func(*test.Tstate, time.Time, interface{}) time.Duration

func initSemaphore(ts *test.Tstate, start time.Time, i interface{}) time.Duration {
	s := i.(*semclnt.SemClnt)
	err := s.Init(0)
	assert.Nil(ts.T, err, "Sem init: %v", err)
	return time.Since(start)
}

func upSemaphore(ts *test.Tstate, start time.Time, i interface{}) time.Duration {
	s := i.(*semclnt.SemClnt)
	err := s.Up()
	assert.Nil(ts.T, err, "Sem up: %v", err)
	return time.Since(start)
}

func downSemaphore(ts *test.Tstate, start time.Time, i interface{}) time.Duration {
	s := i.(*semclnt.SemClnt)
	err := s.Down()
	assert.Nil(ts.T, err, "Sem down: %v", err)
	return time.Since(start)
}

// TODO for matmul, possibly only benchmark internal time
func runProc(ts *test.Tstate, start time.Time, i interface{}) time.Duration {
	p := i.(*proc.Proc)
	err1 := ts.Spawn(p)
	db.DPrintf("TEST1", "Spawned %v", p)
	status, err2 := ts.WaitExit(p.Pid)
	assert.Nil(ts.T, err1, "Failed to Spawn %v", err1)
	assert.Nil(ts.T, err2, "Failed to WaitExit %v", err2)
	// Correctness checks
	assert.True(ts.T, status.IsStatusOK(), "Bad status: %v", status)
	return time.Since(start)
}

func spawnBurstWaitStartProcs(ts *test.Tstate, start time.Time, i interface{}) time.Duration {
	ps := i.([]*proc.Proc)
	spawnBurstProcs(ts, ps)
	waitStartProcs(ts, ps)
	return time.Since(start)
}

// XXX Should get job name in a tuple.
func runMR(ts *test.Tstate, start time.Time, i interface{}) time.Duration {
	ji := i.(*MRJobInstance)
	ji.PrepareMRJob()
	ji.ready <- true
	<-ji.ready
	ji.StartMRJob()
	ji.Wait()
	err := mr.PrintMRStats(ts.FsLib, ji.jobname)
	assert.Nil(ts.T, err, "Error print MR stats: %v", err)
	return time.Since(start)
}

func runKV(ts *test.Tstate, start time.Time, i interface{}) time.Duration {
	ji := i.(*KVJobInstance)
	// Start some balancers
	ji.StartKVJob()
	db.DPrintf("TEST", "Made KV job")
	// Add more kvd groups.
	for i := 0; i < ji.nkvd; i++ {
		ji.AddKVDGroup()
	}
	// Note that we are prepared to run the job.
	ji.ready <- true
	// Wait for an ack.
	<-ji.ready
	db.DPrintf("TEST", "Added KV groups")
	// Run through the job phases.
	for !ji.IsDone() {
		ji.NextPhase()
	}
	ji.Stop()
	db.DPrintf("TEST", "Stopped KV")
	return time.Since(start)
}
