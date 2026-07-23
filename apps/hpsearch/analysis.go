package hpsearch

// Post-hoc analysis of the learning curves a job produces. None of this
// touches SigmaOS -- it is pure arithmetic over []*Curve -- so it is
// unit-testable without booting a realm, and is shared by the baseline and
// live-pruning benchmarks alike.

import (
	"math"
	"time"

	db "sigmaos/debug"
)

// TargetFrac is the time-to-target-accuracy threshold, as a fraction of the
// best score any config reaches.
const TargetFrac = 0.9

// Result summarizes how much compute a baseline hyperparameter search (no
// early stopping) wastes, versus an oracle pruning policy computed post-hoc
// from the logged curves.
type Result struct {
	ActualCoreSeconds float64
	OracleCoreSeconds float64
	WastedCoreSeconds float64
	WastedFrac        float64
	// Best score with every config run to completion, best score still
	// reachable under oracle pruning, and the difference.
	BestQualityAll  float64
	BestQualityKept float64
	QualityLost     float64
	// When the search first reaches TargetFrac*BestQualityAll.
	ItersToTarget   int
	SecsToTarget    float64
	PerConfigCurves []*Curve
}

// LiveResult summarizes a live-pruning run (StartPruningJob). Unlike Result
// there is no oracle to compare against here: these are the numbers the
// causal policy actually achieved, to be set against a Result computed from
// a baseline run of the same config.
type LiveResult struct {
	CoreSeconds float64
	NPruned     int
	BestQuality float64
}

// BestScore returns the highest score in s[:n], or -Inf for n<=0.
func BestScore(s []float64, n int) float64 {
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

// BestQuality returns the highest score reached by any of the curves.
func BestQuality(curves []*Curve) float64 {
	best := math.Inf(-1)
	for _, c := range curves {
		if q := BestScore(c.Scores, len(c.Scores)); q > best {
			best = q
		}
	}
	return best
}

// CoreSeconds returns the total compute the curves represent: one core for
// iterDur per iteration actually run. A pruned curve has a truncated
// Scores, so it contributes only the iterations it got through.
func CoreSeconds(curves []*Curve, iterDur time.Duration) float64 {
	total := 0.0
	for _, c := range curves {
		total += float64(len(c.Scores)) * iterDur.Seconds()
	}
	return total
}

// NumPruned counts how many configs the live pruning policy cut short.
func NumPruned(curves []*Curve) int {
	n := 0
	for _, c := range curves {
		if c.Pruned {
			n++
		}
	}
	return n
}

// FirstIterToTarget returns the earliest iteration at which any curve reaches
// target, and whether any did. A pruned curve has a truncated Scores, so it
// can't reach target past its prune point.
func FirstIterToTarget(curves []*Curve, target float64) (int, bool) {
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

// OraclePruneIters returns, for each curve, the first iteration at which it
// falls behind the best config's score-so-far by more than margin and never
// recovers within margin for the rest of the run -- a simple successive-
// halving-style proxy for an ASHA scheduler. No actual pruning needs to run
// for this baseline; it's computed after the fact from the full curves.
func OraclePruneIters(curves []*Curve, margin float64) []int {
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

// Analyze computes the baseline-versus-oracle summary for a completed
// baseline job: what the run actually cost, what an oracle that got to see
// every completed curve would have spent instead, and what quality that
// oracle would have given up.
func Analyze(curves []*Curve, cfg *Config) *Result {
	pruneIters := OraclePruneIters(curves, cfg.Margin)
	iterSec := cfg.IterDur.Seconds()
	oracle := 0.0
	bestKept := math.Inf(-1)
	for ci, c := range curves {
		oracle += float64(pruneIters[ci]) * iterSec
		if q := BestScore(c.Scores, pruneIters[ci]); q > bestKept {
			bestKept = q
		}
		db.DPrintf(db.HPSEARCH, "hpsearch config %d asymptote %f pruned-at %d/%d", c.ConfigId, c.Asymptote, pruneIters[ci], len(c.Scores))
	}
	actual := CoreSeconds(curves, cfg.IterDur)
	bestAll := BestQuality(curves)
	itersToTarget, _ := FirstIterToTarget(curves, TargetFrac*bestAll)
	return &Result{
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
}

// AnalyzeLive tallies what a live-pruning run actually spent and how many
// of its configs got pruned.
func AnalyzeLive(curves []*Curve, cfg *Config) *LiveResult {
	for _, c := range curves {
		db.DPrintf(db.HPSEARCH, "hpsearch-live config %d asymptote %f pruned %v at %d/%d",
			c.ConfigId, c.Asymptote, c.Pruned, c.PrunedAtIter, cfg.MaxIters)
	}
	return &LiveResult{
		CoreSeconds: CoreSeconds(curves, cfg.IterDur),
		NPruned:     NumPruned(curves),
		BestQuality: BestQuality(curves),
	}
}
