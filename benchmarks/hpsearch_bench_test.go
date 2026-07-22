package benchmarks_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/hpsearch"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

// PruneMargin is how far behind the best config's score a config must fall
// (and stay) before the oracle would have pruned it.
const PruneMargin = 0.1

// TargetFrac sets the time-to-target-accuracy threshold as a fraction of the
// best score any config reaches: the target is TargetFrac * best-achievable.
const TargetFrac = 0.9

// bestScore returns the highest score in s[:n] (the quality reachable if a
// config is stopped after n iterations). Returns -Inf for n<=0.
func bestScore(s []float64, n int) float64 {
	if n > len(s) {
		n = len(s)
	}
	best := math.Inf(-1)
	for i := 0; i < n; i++ {
		if s[i] > best {
			best = s[i]
		}
	}
	return best
}

// firstIterToTarget returns the earliest iteration at which any curve reaches
// target within the iterations it actually ran (len(Scores)), and whether any
// did. A curve pruned early has a truncated Scores, so it can't reach target
// after its prune point.
func firstIterToTarget(curves []*hpsearch.Curve, target float64) (int, bool) {
	best := -1
	for _, c := range curves {
		for i, s := range c.Scores {
			if s >= target {
				if best < 0 || i < best {
					best = i
				}
				break
			}
		}
	}
	return best, best >= 0
}

// HPSearchResult summarizes how much compute a baseline hyperparameter
// search (no early stopping) wastes, versus an oracle pruning policy
// computed post-hoc from the logged curves.
type HPSearchResult struct {
	ActualCoreSeconds float64
	OracleCoreSeconds float64
	WastedCoreSeconds float64
	WastedFrac        float64
	// BestQualityAll is the best score any config reaches running every config
	// to completion; BestQualityKept is the best still reachable if the oracle
	// prunes; QualityLost is what pruning gives up (0 means it kept a winner).
	BestQualityAll  float64
	BestQualityKept float64
	QualityLost     float64
	// ItersToTarget / SecsToTarget: when the search first reaches
	// TargetFrac*BestQualityAll.
	ItersToTarget   int
	SecsToTarget    float64
	PerConfigCurves []*hpsearch.Curve
}

// oraclePruneIters returns, for each curve, the first iteration at which it
// falls behind the best config's score-so-far by more than margin and never
// recovers within margin for the rest of the run -- a simple successive-
// halving-style proxy for an ASHA scheduler. No actual pruning needs to run
// for this baseline; it's computed after the fact from the full curves.
func oraclePruneIters(curves []*hpsearch.Curve, margin float64) []int {
	maxIters := len(curves[0].Scores)
	// Compute the best score across all configs at each iteration.
	bestAtIter := make([]float64, maxIters)
	for i := 0; i < maxIters; i++ {
		best := curves[0].Scores[i]
		for _, c := range curves[1:] {
			if c.Scores[i] > best {
				best = c.Scores[i]
			}
		}
		bestAtIter[i] = best
	}
	pruneIters := make([]int, len(curves))
	// For each config, find the earliest iteration it falls behind for good.
	for ci, c := range curves {
		pruneIters[ci] = maxIters // default: never pruned
		for i := 0; i < maxIters; i++ {
			if c.Scores[i] < bestAtIter[i]-margin && staysBehind(c.Scores, bestAtIter, i, margin) {
				pruneIters[ci] = i
				break
			}
		}
	}
	return pruneIters
}

// staysBehind reports whether scores remains more than margin behind
// bestAtIter for every iteration from i through the end of the run.
func staysBehind(scores, bestAtIter []float64, i int, margin float64) bool {
	for j := i; j < len(scores); j++ {
		if scores[j] >= bestAtIter[j]-margin {
			return false
		}
	}
	return true
}

func TestHPSearchBaseline(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	cfg := DefaultHPSearchBenchConfig()
	ji := NewHPSearchJobInstance(mrts.GetRealm(REALM1), cfg)
	err = ji.StartHPSearchJob()
	assert.Nil(t, err, "Error StartHPSearchJob: %v", err)

	// Run every config to completion (no pruning) and collect its curve.
	curves, err := ji.WaitJobExit()
	assert.Nil(t, err, "Error WaitJobExit: %v", err)
	assert.Equal(t, cfg.NConfigs, len(curves))

	// Compute, post-hoc, where an oracle would have pruned each config.
	pruneIters := oraclePruneIters(curves, PruneMargin)

	actual := 0.0
	oracle := 0.0
	bestAll := math.Inf(-1)
	bestKept := math.Inf(-1)
	iterSec := cfg.IterDur.Seconds()
	for ci, c := range curves {
		actual += float64(len(c.Scores)) * iterSec
		oracle += float64(pruneIters[ci]) * iterSec
		if q := bestScore(c.Scores, len(c.Scores)); q > bestAll {
			bestAll = q
		}
		if q := bestScore(c.Scores, pruneIters[ci]); q > bestKept {
			bestKept = q
		}
		db.DPrintf(db.HPSEARCH, "hpsearch config %d asymptote %f pruned-at %d/%d", c.ConfigId, c.Asymptote, pruneIters[ci], len(c.Scores))
	}
	itersToTarget, _ := firstIterToTarget(curves, TargetFrac*bestAll)

	res := &HPSearchResult{
		ActualCoreSeconds: actual,
		OracleCoreSeconds: oracle,
		WastedCoreSeconds: actual - oracle,
		WastedFrac:        (actual - oracle) / actual,
		BestQualityAll:    bestAll,
		BestQualityKept:   bestKept,
		QualityLost:       bestAll - bestKept,
		ItersToTarget:     itersToTarget,
		SecsToTarget:      float64(itersToTarget) * iterSec,
		PerConfigCurves:   curves,
	}

	db.DPrintf(db.ALWAYS, "HPSearch baseline: actual %.2f core-s, oracle %.2f core-s, wasted %.2f core-s (%.1f%%)",
		res.ActualCoreSeconds, res.OracleCoreSeconds, res.WastedCoreSeconds, res.WastedFrac*100)
	db.DPrintf(db.ALWAYS, "HPSearch baseline quality: best-all %.3f, best-kept-under-oracle %.3f, quality lost %.3f; time-to-target (%.0f%% of best) %d iters / %.2f core-s",
		res.BestQualityAll, res.BestQualityKept, res.QualityLost, TargetFrac*100, res.ItersToTarget, res.SecsToTarget)

	assert.True(t, res.WastedFrac > 0, "Expected oracle pruning to save some compute, saved %v", res.WastedFrac)
	assert.True(t, res.WastedFrac < 1, "Oracle saved 100%% of compute, which shouldn't be possible")
}
