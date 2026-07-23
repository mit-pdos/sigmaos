package benchmarks_test

// The two hpsearch benchmarks. Everything these tests do beyond booting a
// realm and asserting lives in apps/hpsearch: starting and reaping a job
// (StartJob, StartPruningJob, Job.Wait) and analyzing the curves it
// produces (Analyze, AnalyzeLive).

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/hpsearch"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

// runJob starts a baseline search and waits for it to finish. Neither
// benchmark here adds contention while the search runs, so they use this
// rather than driving StartJob/Job.Wait themselves.
func runJob(sc *sigmaclnt.SigmaClnt, cfg *hpsearch.Config) ([]*hpsearch.Curve, error) {
	j, err := hpsearch.StartJob(sc, cfg)
	if err != nil {
		return nil, err
	}
	return j.Wait()
}

// runPruningJob starts a live-pruning search and waits for it to finish,
// exactly like runJob but with the hp-trainer-pruned variant.
func runPruningJob(sc *sigmaclnt.SigmaClnt, cfg *hpsearch.Config) ([]*hpsearch.Curve, error) {
	j, err := hpsearch.StartPruningJob(sc, cfg)
	if err != nil {
		return nil, err
	}
	return j.Wait()
}

func TestHPSearchBaseline(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	cfg := hpsearch.DefaultConfig()

	// Run every config to completion (no pruning) and collect its curve.
	curves, err := runJob(mrts.GetRealm(REALM1).SigmaClnt, cfg)
	if !assert.Nil(t, err, "Error runJob: %v", err) {
		return
	}
	assert.Equal(t, cfg.NConfigs, len(curves))

	// Compute, post-hoc, where an oracle would have pruned each config.
	res := hpsearch.Analyze(curves, cfg)

	db.DPrintf(db.ALWAYS, "HPSearch baseline: actual %.2f core-s, oracle %.2f core-s, wasted %.2f core-s (%.1f%%)",
		res.ActualCoreSeconds, res.OracleCoreSeconds, res.WastedCoreSeconds, res.WastedFrac*100)
	db.DPrintf(db.ALWAYS, "HPSearch baseline quality: best-all %.3f, best-kept-under-oracle %.3f, quality lost %.3f; time-to-target (%.0f%% of best) %d iters / %.2f core-s",
		res.BestQualityAll, res.BestQualityKept, res.QualityLost, hpsearch.TargetFrac*100, res.ItersToTarget, res.SecsToTarget)

	assert.True(t, res.WastedFrac > 0, "Expected oracle pruning to save some compute, saved %v", res.WastedFrac)
	assert.True(t, res.WastedFrac < 1, "Oracle saved 100%% of compute, which shouldn't be possible")
}

// TestHPSearchLivePruning compares three numbers for the same synthetic
// hyperparameter search: the baseline (every config runs to completion,
// TestHPSearchBaseline), the post-hoc oracle's ceiling (OraclePruneIters,
// which gets to look at every config's completed curve), and a real,
// causal live-pruning policy (hp-trainer-pruned, via runPruningJob) that
// only ever sees scores siblings have already published. Unlike the oracle,
// the live policy has no guarantee of never pruning a config that would
// have caught up later, so it isn't expected to land between the baseline
// and the oracle -- it's a real, achievable number to set against that
// theoretical ceiling.
func TestHPSearchLivePruning(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	cfg := hpsearch.DefaultConfig()
	sc := mrts.GetRealm(REALM1).SigmaClnt

	// Baseline run: gives the full curves needed to compute the oracle's
	// ceiling, and the "actual" cost of running everything to completion.
	baseCurves, err := runJob(sc, cfg)
	if !assert.Nil(t, err, "Error runJob (baseline): %v", err) {
		return
	}
	assert.Equal(t, cfg.NConfigs, len(baseCurves))
	base := hpsearch.Analyze(baseCurves, cfg)

	// Live-pruning run: same config/seeds, but trainers prune themselves
	// online against each other's published progress.
	liveCurves, err := runPruningJob(sc, cfg)
	if !assert.Nil(t, err, "Error runPruningJob (live): %v", err) {
		return
	}
	assert.Equal(t, cfg.NConfigs, len(liveCurves))
	live := hpsearch.AnalyzeLive(liveCurves, cfg)

	// Both runs use the same absolute target so their times are comparable.
	target := hpsearch.TargetFrac * base.BestQualityAll
	baseIters, _ := hpsearch.FirstIterToTarget(baseCurves, target)
	liveIters, liveHit := hpsearch.FirstIterToTarget(liveCurves, target)

	db.DPrintf(db.ALWAYS, "HPSearch live pruning: actual %.2f core-s, live-pruned %.2f core-s (saved %.1f%%), oracle %.2f core-s (saved %.1f%%), %d/%d configs pruned",
		base.ActualCoreSeconds, live.CoreSeconds, (base.ActualCoreSeconds-live.CoreSeconds)/base.ActualCoreSeconds*100,
		base.OracleCoreSeconds, base.WastedFrac*100, live.NPruned, cfg.NConfigs)
	db.DPrintf(db.ALWAYS, "HPSearch live pruning quality: best-all %.3f, best-live-kept %.3f, quality lost %.3f; time-to-target (%.0f%% of best) baseline %d iters vs live %d iters (live reached target: %v)",
		base.BestQualityAll, live.BestQuality, base.BestQualityAll-live.BestQuality, hpsearch.TargetFrac*100, baseIters, liveIters, liveHit)

	// A sanity check only: every pruned curve is a strict prefix of the full
	// one, so live.CoreSeconds <= base.ActualCoreSeconds holds by construction.
	assert.True(t, live.CoreSeconds <= base.ActualCoreSeconds, "Live-pruned run used more compute than the baseline")
	assert.True(t, live.NPruned > 0, "Expected the live policy to prune at least one config")
}
