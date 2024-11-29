package benchmarks_test

import (
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	db "sigmaos/debug"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/semclnt"
	sp "sigmaos/sigmap"
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
	assert.Nil(ts.Ts.T, err, "Sem init: %v", err)
	return time.Since(start), 1.0
}

func upSemaphore(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	s := i.(*semclnt.SemClnt)
	err := s.Up()
	assert.Nil(ts.Ts.T, err, "Sem up: %v", err)
	return time.Since(start), 1.0
}

func downSemaphore(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	s := i.(*semclnt.SemClnt)
	err := s.Down()
	assert.Nil(ts.Ts.T, err, "Sem down: %v", err)
	return time.Since(start), 1.0
}

func warmupRealmBench(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	prog := i.(string)
	start, nDL := warmupRealm(ts, []string{prog})
	return time.Since(start), float64(nDL)
}

// TODO for matmul, possibly only benchmark internal time
func runProc(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	start := time.Now()
	p := i.(*proc.Proc)
	err1 := ts.Spawn(p)
	db.DPrintf(db.TEST1, "Spawned %v", p)
	status, err2 := ts.WaitExit(p.GetPid())
	assert.Nil(ts.Ts.T, err1, "Failed to Spawn %v", err1)
	assert.Nil(ts.Ts.T, err2, "Failed to WaitExit %v", err2)
	// Correctness checks
	assert.True(ts.Ts.T, status.IsStatusOK(), "Bad status: %v", status)
	return time.Since(start), 1.0
}

func spawnWaitStartProc(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	p := i.(*proc.Proc)
	ps := []*proc.Proc{p}
	start := time.Now()
	spawnProcs(ts, ps)
	waitStartProcs(ts, ps)
	return time.Since(start), 1.0
}

func spawnWaitStartProcs(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ps := i.([]*proc.Proc)
	start := time.Now()
	spawnProcs(ts, ps)
	waitStartProcs(ts, ps)
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
	ji := i.(*MRJobInstance)
	ji.PrepareMRJob()
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	sdc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
	sdc.MonitorMSchedStats(ts.GetRealm(), SCHEDD_STAT_MONITOR_PERIOD)
	defer sdc.Done()
	start := time.Now()
	db.DPrintf(db.BENCH, "Start MR job")
	ji.StartMRJob()
	ji.Wait()
	db.DPrintf(db.BENCH, "Done MR job")
	dur := time.Since(start)
	ji.WaitJobExit()
	err := mr.PrintMRStats(ts.FsLib, ji.jobRoot, ji.jobname)
	assert.Nil(ts.Ts.T, err, "Error print MR stats: %v", err)
	// Sleep a bit to allow util to update.
	time.Sleep(4 * time.Second)
	ji.p.Done()
	return dur, 1.0
}

func runKV(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*KVJobInstance)
	rpcc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
	rpcc.MonitorMSchedStats(ts.GetRealm(), SCHEDD_STAT_MONITOR_PERIOD)
	defer rpcc.Done()
	// Start some balancers
	start := time.Now()
	ji.StartKVJob()

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

func runCached(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*CachedJobInstance)
	ji.ready <- true
	<-ji.ready
	start := time.Now()
	ji.RunCachedJob()
	return time.Since(start), 1.0
}

// XXX Should get job name in a tuple.
func runWww(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*WwwJobInstance)
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	rpcc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
	rpcc.MonitorMSchedStats(ts.GetRealm(), SCHEDD_STAT_MONITOR_PERIOD)
	defer rpcc.Done()
	start := time.Now()
	ji.StartWwwJob()
	ji.Wait()
	return time.Since(start), 1.0
}

func runMSched(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*MSchedJobInstance)
	ji.ready <- true
	<-ji.ready
	start := time.Now()
	ji.StartMSchedJob()
	ji.Wait()
	return time.Since(start), 1.0
}

func runHotel(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*HotelJobInstance)
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	if ji.sigmaos {
		rpcc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
		rpcc.MonitorMSchedStats(ts.GetRealm(), SCHEDD_STAT_MONITOR_PERIOD)
		defer rpcc.Done()
	}
	start := time.Now()
	ji.StartHotelJob()
	ji.Wait()
	return time.Since(start), 1.0
}

func runSocialNetwork(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*SocialNetworkJobInstance)
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	if ji.sigmaos {
		rpcc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
		rpcc.MonitorMSchedStats(ts.GetRealm(), SCHEDD_STAT_MONITOR_PERIOD)
		defer rpcc.Done()
	}
	start := time.Now()
	ji.StartSocialNetworkJob()
	ji.Wait()
	return time.Since(start), 1.0
}

func runImgResize(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*ImgResizeJobInstance)
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	if ji.sigmaos {
		rpcc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
		rpcc.MonitorMSchedStats(ts.GetRealm(), SCHEDD_STAT_MONITOR_PERIOD)
		defer rpcc.Done()
	}
	//	ji.Cleanup()
	start := time.Now()
	ji.StartImgResizeJob()
	ji.Wait()
	t := time.Since(start)
	time.Sleep(2 * time.Second)
	db.DPrintf(db.TEST, "[%v] Cleaning up imgresize", ts.GetRealm())
	//	ji.Cleanup()
	db.DPrintf(db.TEST, "[%v] Done cleaning up imgresize", ts.GetRealm())
	return t, 1.0
}

func runImgResizeRPC(ts *test.RealmTstate, i interface{}) (time.Duration, float64) {
	ji := i.(*ImgResizeRPCJobInstance)
	ji.ready <- true
	<-ji.ready
	// Start a procd clnt, and monitor procds
	if ji.sigmaos {
		rpcc := mschedclnt.NewMSchedClnt(ts.SigmaClnt.FsLib, sp.NOT_SET)
		rpcc.MonitorMSchedStats(ts.GetRealm(), SCHEDD_STAT_MONITOR_PERIOD)
		defer rpcc.Done()
	}
	//	ji.Cleanup()
	start := time.Now()
	ji.StartImgResizeRPCJob()
	ji.Wait()
	t := time.Since(start)
	time.Sleep(2 * time.Second)
	db.DPrintf(db.TEST, "[%v] Cleaning up imgresize", ts.GetRealm())
	//	ji.Cleanup()
	db.DPrintf(db.TEST, "[%v] Done cleaning up imgresize", ts.GetRealm())
	return t, 1.0
}
