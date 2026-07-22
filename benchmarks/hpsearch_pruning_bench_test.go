package benchmarks_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

// TestHPSearchLivePruning compares three numbers for the same synthetic
// hyperparameter search: the baseline (every config runs to completion,
// TestHPSearchBaseline), the post-hoc oracle's ceiling (oraclePruneIters,
// which gets to look at every config's completed curve), and a real,
// causal live-pruning policy (hp-trainer-pruned, via
// StartHPSearchPruningJob) that only ever sees scores siblings have already
// published. Unlike the oracle, the live policy has no guarantee of never
// pruning a config that would have caught up later, so it isn't expected to
// land between the baseline and the oracle -- it's a real, achievable
// number to set against that theoretical ceiling.
func TestHPSearchLivePruning(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	cfg := DefaultHPSearchBenchConfig()
	iterSec := cfg.IterDur.Seconds()

	// Baseline run: gives the full curves needed to compute the oracle's
	// ceiling, and the "actual" cost of running everything to completion.
	baseline := NewHPSearchJobInstance(mrts.GetRealm(REALM1), cfg)
	err = baseline.StartHPSearchJob()
	assert.Nil(t, err, "Error StartHPSearchJob: %v", err)
	baseCurves, err := baseline.WaitJobExit()
	assert.Nil(t, err, "Error WaitJobExit (baseline): %v", err)
	assert.Equal(t, cfg.NConfigs, len(baseCurves))

	pruneIters := oraclePruneIters(baseCurves, PruneMargin)
	actual := 0.0
	oracle := 0.0
	bestAll := math.Inf(-1) // best quality reachable running every config fully
	for ci, c := range baseCurves {
		actual += float64(len(c.Scores)) * iterSec
		oracle += float64(pruneIters[ci]) * iterSec
		if q := bestScore(c.Scores, len(c.Scores)); q > bestAll {
			bestAll = q
		}
	}

	// Live-pruning run: same config/seeds, but trainers prune themselves
	// online against each other's published progress.
	live := NewHPSearchJobInstance(mrts.GetRealm(REALM1), cfg)
	progressDir, err := live.StartHPSearchPruningJob(PruneMargin)
	assert.Nil(t, err, "Error StartHPSearchPruningJob: %v", err)
	// Clean up the shared progress directory once the job is done.
	defer func() {
		live.RmDirEntries(progressDir)
		live.RmDir(progressDir)
	}()

	liveCurves, err := live.WaitJobExit()
	assert.Nil(t, err, "Error WaitJobExit (live): %v", err)
	assert.Equal(t, cfg.NConfigs, len(liveCurves))

	liveActual := 0.0
	nPruned := 0
	bestLive := math.Inf(-1) // best quality the live-pruned run actually kept
	// Tally real compute used and how many configs actually got pruned.
	for _, c := range liveCurves {
		liveActual += float64(len(c.Scores)) * iterSec
		if c.Pruned {
			nPruned++
		}
		if q := bestScore(c.Scores, len(c.Scores)); q > bestLive {
			bestLive = q
		}
		db.DPrintf(db.HPSEARCH, "hpsearch-live config %d asymptote %f pruned %v at %d/%d",
			c.ConfigId, c.Asymptote, c.Pruned, c.PrunedAtIter, cfg.MaxIters)
	}

	// Time-to-target uses the same absolute target for both runs so they're
	// comparable: does aggressive live pruning delay or miss reaching it?
	target := TargetFrac * bestAll
	baseIters, _ := firstIterToTarget(baseCurves, target)
	liveIters, liveHit := firstIterToTarget(liveCurves, target)

	db.DPrintf(db.ALWAYS, "HPSearch live pruning: actual %.2f core-s, live-pruned %.2f core-s (saved %.1f%%), oracle %.2f core-s (saved %.1f%%), %d/%d configs pruned",
		actual, liveActual, (actual-liveActual)/actual*100, oracle, (actual-oracle)/actual*100, nPruned, cfg.NConfigs)
	db.DPrintf(db.ALWAYS, "HPSearch live pruning quality: best-all %.3f, best-live-kept %.3f, quality lost %.3f; time-to-target (%.0f%% of best) baseline %d iters vs live %d iters (live reached target: %v)",
		bestAll, bestLive, bestAll-bestLive, TargetFrac*100, baseIters, liveIters, liveHit)

	// liveActual <= actual holds by construction (every pruned curve is a
	// strict prefix of the full one), so this is a sanity check, not a
	// meaningful result on its own -- the DPrintf line above is the result.
	assert.True(t, liveActual <= actual, "Live-pruned run used more compute than the baseline")
	assert.True(t, nPruned > 0, "Expected the live policy to prune at least one config")
}
