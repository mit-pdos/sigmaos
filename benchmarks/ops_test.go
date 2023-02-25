package benchmarks_test

import (
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/mr"
	"sigmaos/proc"
	"sigmaos/scheddclnt"
	"sigmaos/semclnt"
	"sigmaos/test"
)

//
// The set of basic operations that we benchmark.
//

type testOp func(*test.RealmTstate, interface{}) (time.Duration, float64)

func initSemaphore(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	s := i.(*semclnt.SemClnt)
	err := s.Init(0)
	assert.Nil(ts.T, err, "Sem init: %v", err)
	return time.Since(start), 1.0
}

func upSemaphore(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	s := i.(*semclnt.SemClnt)
	err := s.Up()
	assert.Nil(ts.T, err, "Sem up: %v", err)
	return time.Since(start), 1.0
}

func downSemaphore(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	s := i.(*semclnt.SemClnt)
	err := s.Down()
	assert.Nil(ts.T, err, "Sem down: %v", err)
	return time.Since(start), 1.0
}

// TODO for matmul, possibly only benchmark internal time
func runProc(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	p := i.(*proc.Proc)
	err1 := ts.Spawn(p)
	db.DPrintf(db.TEST1, "Spawned %v", p)
	status, err2 := ts.WaitExit(p.GetPid())
	assert.Nil(ts.T, err1, "Failed to Spawn %v", err1)
	assert.Nil(ts.T, err2, "Failed to WaitExit %v", err2)
	// Correctness checks
	assert.True(ts.T, status.IsStatusOK(), "Bad status: %v", status)
	return time.Since(start), 1.0
}

func spawnBurstWaitStartProcs(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ps := i.([]*proc.Proc)
	per := len(ps) / N_THREADS
	db.DPrintf(db.ALWAYS, "%v procs per thread", per)
	start := time.Now()
	done := make(chan bool)
	for i := 0; i < N_THREADS; i++ {
		go func(i int) {
			spawnBurstProcs(ts, ps[i*per:(i+1)*per])
			waitStartProcs(ts, ps[i*per:(i+1)*per])
			done <- true
		}(i)
	}
	for i := 0; i < N_THREADS; i++ {
		<-done
	}
	return time.Since(start), 1.0
}

func invokeWaitStartLambdas(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	sems := i.([]*semclnt.SemClnt)
	for _, sem := range sems {
		// Spawn a lambda, which will Up this semaphore when it starts.
		go func(sem *semclnt.SemClnt) {
			spawnLambda(ts, sem.GetPath())
		}(sem)
	}
	for _, sem := range sems {
		// Wait for all the lambdas to start.
		downSemaphore(ts, sem)
	}
	return time.Since(start), 1.0
}

func invokeWaitStartOneLambda(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	sem := i.(*semclnt.SemClnt)
	go func(sem *semclnt.SemClnt) {
		spawnLambda(ts, sem.GetPath())
	}(sem)
	downSemaphore(ts, sem)
	return time.Since(start), 1.0
}

func runMR(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	ji := i.(*MRJobInstance)
	ji.PrepareMRJob()
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	sdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt, ts.GetRealm())
	sdc.MonitorSchedds()
	defer sdc.Done()
	ji.StartMRJob()
	ji.Wait()
	err := mr.PrintMRStats(ts.FsLib, ji.jobname)
	assert.Nil(ts.T, err, "Error print MR stats: %v", err)
	return time.Since(start), 1.0
}

func runKV(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*KVJobInstance)
	pdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt, ts.GetRealm())
	pdc.MonitorSchedds()
	defer pdc.Done()
	// Start some balancers
	start := time.Now()
	ji.StartKVJob()
	db.DPrintf(db.TEST, "Made KV job")
	// Add more kvd groups.
	for i := 0; i < ji.nkvd-1; i++ {
		ji.AddKVDGroup()
	}
	// If not running against redis.
	if !ji.redis {
		cnts := ji.GetKeyCountsPerGroup()
		db.DPrintf(db.ALWAYS, "Key counts per group: %v", cnts)
	}
	// Note that we are prepared to run the job.
	ji.ready <- true
	// Wait for an ack.
	<-ji.ready
	db.DPrintf(db.TEST, "Added KV groups")
	db.DPrintf(db.TEST, "Running clerks")
	// Run through the job phases.
	for !ji.IsDone() {
		ji.NextPhase()
	}
	ji.Stop()
	db.DPrintf(db.TEST, "Stopped KV")
	return time.Since(start), 1.0
}

// XXX Should get job name in a tuple.
func runWww(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*WwwJobInstance)
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	pdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt, ts.GetRealm())
	pdc.MonitorSchedds()
	defer pdc.Done()
	start := time.Now()
	ji.StartWwwJob()
	ji.Wait()
	return time.Since(start), 1.0
}

func runHotel(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*HotelJobInstance)
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	if ji.sigmaos {
		pdc := scheddclnt.MakeScheddClnt(ts.SigmaClnt, ts.GetRealm())
		pdc.MonitorSchedds()
		defer pdc.Done()
	}
	start := time.Now()
	ji.StartHotelJob()
	ji.Wait()
	return time.Since(start), 1.0
}
