// Package hpsearch implements a minimal synthetic hyperparameter-search
// trainer app, used to establish a baseline measurement of how much compute
// today's SigmaOS wastes by running every configuration to completion with
// no early-stopping/pruning policy.
package hpsearch

import (
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/mitchellh/mapstructure"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

const (
	// Range of the per-config asymptotic (converged) score, sampled
	// uniformly from [AsymptoteMin, AsymptoteMin+AsymptoteRange).
	AsymptoteMin   = 0.5
	AsymptoteRange = 0.5
	// Range of the per-config convergence rate.
	KMin   = 0.05
	KRange = 0.15
	// Stddev of the per-iteration noise added to the score.
	NoiseStd = 0.02
)

// Curve is the synthetic learning curve produced by a single hyperparameter
// configuration's trainer proc. It is returned to the caller via
// proc.Status's StatusData field (the same mechanism apps/mr uses for its
// Result struct), so proc/status.go needs no changes.
type Curve struct {
	ConfigId  int
	Seed      int64
	Asymptote float64
	Scores    []float64
}

// NewCurve decodes a Curve back out of a proc.Status's StatusData, which
// arrives as a generic map[string]interface{} after the JSON round-trip
// through sigmap.
func NewCurve(data interface{}) (*Curve, error) {
	c := &Curve{}
	err := mapstructure.Decode(data, c)
	return c, err
}

// syntheticCurve deterministically generates a per-config learning curve
// from a seed: score(i) = asymptote*(1-exp(-k*i)) + noise. Asymptote and
// convergence rate k are themselves derived from the seed, so different
// configs plateau at different scores and at different speeds.
func syntheticCurve(seed int64, maxIters int) (asymptote float64, scores []float64) {
	rng := rand.New(rand.NewSource(seed))
	asymptote = AsymptoteMin + AsymptoteRange*rng.Float64()
	k := KMin + KRange*rng.Float64()
	scores = make([]float64, maxIters)
	for i := 0; i < maxIters; i++ {
		scores[i] = asymptote*(1-math.Exp(-k*float64(i))) + rng.NormFloat64()*NoiseStd
	}
	return asymptote, scores
}

// RunTrainer is the entry point for the hp-trainer proc. Args are
// [configId, seed, maxIters, iterDurMs]. It always runs all maxIters
// iterations (no early stopping) — that is the point of this baseline.
func RunTrainer(args []string) {
	if len(args) != 4 {
		db.DFatalf("RunTrainer: wrong number of args %v", args)
	}
	configId, err := strconv.Atoi(args[0])
	if err != nil {
		db.DFatalf("RunTrainer: configId %v not an int: %v", args[0], err)
	}
	seed, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		db.DFatalf("RunTrainer: seed %v not an int: %v", args[1], err)
	}
	maxIters, err := strconv.Atoi(args[2])
	if err != nil {
		db.DFatalf("RunTrainer: maxIters %v not an int: %v", args[2], err)
	}
	iterMs, err := strconv.Atoi(args[3])
	if err != nil {
		db.DFatalf("RunTrainer: iterDurMs %v not an int: %v", args[3], err)
	}

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("RunTrainer: NewSigmaClnt err %v", err)
	}
	if err := sc.Started(); err != nil {
		db.DFatalf("RunTrainer: Started err %v", err)
	}

	asymptote, scores := syntheticCurve(seed, maxIters)
	iterDur := time.Duration(iterMs) * time.Millisecond
	for i := range scores {
		time.Sleep(iterDur)
		db.DPrintf(db.HPSEARCH, "hp-trainer config %d iter %d score %f", configId, i, scores[i])
	}

	curve := Curve{ConfigId: configId, Seed: seed, Asymptote: asymptote, Scores: scores}
	sc.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK", curve))
}
