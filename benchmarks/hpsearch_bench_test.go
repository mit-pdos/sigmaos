package benchmarks_test

import (
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

// HPSearchResult summarizes how much compute a baseline hyperparameter
// search (no early stopping) wastes, versus an oracle pruning policy
// computed post-hoc from the logged curves.
type HPSearchResult struct {
	ActualCoreSeconds float64
	OracleCoreSeconds float64
	WastedCoreSeconds float64
	WastedFrac        float64
	PerConfigCurves   []*hpsearch.Curve
}

// oraclePruneIters returns, for each curve, the first iteration at which it
// falls behind the best config's score-so-far by more than margin and never
// recovers within margin for the rest of the run -- a simple successive-
// halving-style proxy for an ASHA scheduler. No actual pruning needs to run
// for this baseline; it's computed after the fact from the full curves.
func oraclePruneIters(curves []*hpsearch.Curve, margin float64) []int {
	maxIters := len(curves[0].Scores)
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

	curves, err := ji.WaitJobExit()
	assert.Nil(t, err, "Error WaitJobExit: %v", err)
	assert.Equal(t, cfg.NConfigs, len(curves))

	pruneIters := oraclePruneIters(curves, PruneMargin)

	actual := 0.0
	oracle := 0.0
	iterSec := cfg.IterDur.Seconds()
	for ci, c := range curves {
		actual += float64(len(c.Scores)) * iterSec
		oracle += float64(pruneIters[ci]) * iterSec
		db.DPrintf(db.HPSEARCH, "hpsearch config %d asymptote %f pruned-at %d/%d", c.ConfigId, c.Asymptote, pruneIters[ci], len(c.Scores))
	}

	res := &HPSearchResult{
		ActualCoreSeconds: actual,
		OracleCoreSeconds: oracle,
		WastedCoreSeconds: actual - oracle,
		WastedFrac:        (actual - oracle) / actual,
		PerConfigCurves:   curves,
	}

	db.DPrintf(db.ALWAYS, "HPSearch baseline: actual %.2f core-s, oracle %.2f core-s, wasted %.2f core-s (%.1f%%)",
		res.ActualCoreSeconds, res.OracleCoreSeconds, res.WastedCoreSeconds, res.WastedFrac*100)

	assert.True(t, res.WastedFrac > 0, "Expected oracle pruning to save some compute, saved %v", res.WastedFrac)
	assert.True(t, res.WastedFrac < 1, "Oracle saved 100%% of compute, which shouldn't be possible")
}
