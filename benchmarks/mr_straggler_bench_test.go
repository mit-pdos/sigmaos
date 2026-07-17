package benchmarks_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/rand"
	"sigmaos/util/spstats"
)

const (
	// StragglerSlowTaskId is the map task chosen as the artificial straggler
	// (task ids are 0-indexed, so this is always a valid task regardless of
	// how many map tasks the job splits into).
	StragglerSlowTaskId = 0
	// StragglerSlowdownMs is how much extra time the straggler task takes,
	// on top of however long it would normally take to run. Large enough
	// that it clearly dominates this environment's natural task-time
	// variance (S3 read latency, local machine contention).
	StragglerSlowdownMs = 60_000
)

// collectMRStats mirrors apps/mr/mr_test.go's collectStats: it decodes each
// coordinator's final AStat snapshot (Nfail/Nrestart/etc, see
// apps/mr/coord.go) out of the generic ProcStatus data, keeping only the
// snapshot from whichever coordinator generation actually did the work.
func collectMRStats(t *testing.T, stati []*procgroupmgr.ProcStatus) *spstats.TcounterSnapshot {
	mrst := spstats.NewTcounterSnapshot()
	for _, st := range stati {
		if st.IsStatusOK() {
			stro, err := spstats.UnmarshalTcounterSnapshot(st.Data())
			assert.Nil(t, err)
			if stro.Counters["Nmap"] > 0 || stro.Counters["Nreduce"] > 0 {
				mrst = stro
			}
		}
	}
	return mrst
}

// runMRStragglerJob runs a single MR job (optionally with a straggler task
// injected, when slowdownMs > 0, and/or classic speculative execution
// enabled, when specEnabled) to completion and returns the total job
// completion time plus the coordinator's final stats.
func runMRStragglerJob(mrts *test.MultiRealmTstate, slowdownMs int, specEnabled bool) (time.Duration, *spstats.TcounterSnapshot) {
	ts := mrts.T
	p := newRealmPerf(mrts.GetRealm(REALM1))
	defer p.Done()

	// Each call needs its own job name so that InitCoordFS's MkDir doesn't
	// collide with a stale job dir from a previous run.
	jobname := MR_APP + "-mr-straggler-" + rand.String(3) + "-" + mrts.GetRealm(REALM1).GetRealm().String()
	ji := NewMRStragglerJobInstance(mrts.GetRealm(REALM1), p, MR_APP, chooseMRJobRoot(mrts.GetRealm(REALM1)),
		jobname, proc.Tmem(MR_MEM_REQ), StragglerSlowTaskId, slowdownMs, specEnabled)
	ji.PrepareMRJob()

	start := time.Now()
	ji.StartMRJob()
	ji.Wait()
	dur := time.Since(start)
	stati := ji.cm.WaitGroup()

	err := mr.PrintMRStats(ji.FsLib, ji.jobRoot, ji.jobname)
	assert.Nil(ts, err, "Error print MR stats: %v", err)

	return dur, collectMRStats(ts, stati)
}

// TestMRNoStraggler is the control run: same job as TestMRStragglerBaseline,
// but with the straggler disabled (slowdownMs 0), establishing the
// uninjected completion time to compare against.
func TestMRNoStraggler(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(1, benchmarks.E2E)
	dur, mrst := runMRStragglerJob(mrts, 0, false)
	rs.Append(dur, 1.0)
	printResultSummary(rs)

	db.DPrintf(db.ALWAYS, "MR no straggler: completion time %v, stats %v", dur, mrst)
	assert.Equal(t, int64(0), mrst.Counters["Nfail"], "Expected no task failures without a straggler")
	assert.Equal(t, int64(0), mrst.Counters["Nrestart"], "Expected no task restarts without a straggler")
}

// TestMRStragglerBaseline injects a single artificially-slow map task
// (StragglerSlowTaskId, delayed by StragglerSlowdownMs) and measures how much
// it hurts total job completion time compared to TestMRNoStraggler. It also
// confirms that Nfail/Nrestart stay 0 -- today's MR coordinator has no
// straggler detection at all, so a slow task is never treated as a failure,
// it just silently drags out the job.
func TestMRStragglerBaseline(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(1, benchmarks.E2E)
	dur, mrst := runMRStragglerJob(mrts, StragglerSlowdownMs, false)
	rs.Append(dur, 1.0)
	printResultSummary(rs)

	db.DPrintf(db.ALWAYS, "MR straggler (task %d +%dms): completion time %v, stats %v", StragglerSlowTaskId, StragglerSlowdownMs, dur, mrst)
	assert.Equal(t, int64(0), mrst.Counters["Nfail"], "Straggler task shouldn't be treated as a failure")
	assert.Equal(t, int64(0), mrst.Counters["Nrestart"], "Straggler task shouldn't be treated as a failure")
	assert.True(t, dur >= time.Duration(StragglerSlowdownMs)*time.Millisecond, "Job completion time should reflect the injected straggler delay")
}

// TestMRSpeculativeExecution runs the same straggler-injected job as
// TestMRStragglerBaseline, but with classic speculative execution enabled
// (apps/mr/coord.go's speculate/speculateMap/speculateReduce). Once most of
// the map phase is done, the coordinator should notice the artificially-slow
// task is far behind the average and launch a backup execution of it,
// letting whichever attempt (original or backup) finishes first win --
// recovering most of the straggler's cost.
//
// This is deliberately its own standalone test (own fresh realm), rather
// than running back-to-back with a no-mitigation baseline job in the same
// test/realm: running two large MR jobs sequentially in one realm reliably
// wedges the scheduler (see BUG.md) for reasons unrelated to this code, so
// compare this test's printed completion time against
// TestMRStragglerBaseline's separately-printed one instead of asserting a
// direct in-process comparison.
func TestMRSpeculativeExecution(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rs := benchmarks.NewResults(1, benchmarks.E2E)
	dur, mrst := runMRStragglerJob(mrts, StragglerSlowdownMs, true)
	rs.Append(dur, 1.0)
	printResultSummary(rs)

	db.DPrintf(db.ALWAYS, "MR straggler (task %d +%dms) with speculative execution: completion time %v, stats %v -- compare against TestMRStragglerBaseline's printed completion time", StragglerSlowTaskId, StragglerSlowdownMs, dur, mrst)
	assert.Equal(t, int64(0), mrst.Counters["Nfail"], "Straggler task shouldn't be treated as a failure")
	assert.Equal(t, int64(0), mrst.Counters["Nrestart"], "Straggler task shouldn't be treated as a failure")
	assert.True(t, mrst.Counters["Nspeculate"] > 0, "Expected speculative execution to back up the straggler task")
}
