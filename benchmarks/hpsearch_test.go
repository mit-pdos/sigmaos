package benchmarks_test

import (
	"fmt"
	"time"

	"sigmaos/apps/hpsearch"
	"sigmaos/proc"
	"sigmaos/test"
)

// HPSearchBenchConfig is kept local to this benchmark (rather than added to
// the shared benchmarks.Config machinery in config.go/config_test.go) since
// it doesn't extend anything that already lives there.
type HPSearchBenchConfig struct {
	NConfigs int
	MaxIters int
	IterDur  time.Duration
	Mcpu     proc.Tmcpu
	Seed     int64
}

func DefaultHPSearchBenchConfig() *HPSearchBenchConfig {
	return &HPSearchBenchConfig{
		NConfigs: 8,
		MaxIters: 20,
		IterDur:  50 * time.Millisecond,
		Mcpu:     1000,
		Seed:     1,
	}
}

type HPSearchJobInstance struct {
	*test.RealmTstate
	cfg   *HPSearchBenchConfig
	procs []*proc.Proc
}

func NewHPSearchJobInstance(ts *test.RealmTstate, cfg *HPSearchBenchConfig) *HPSearchJobInstance {
	return &HPSearchJobInstance{RealmTstate: ts, cfg: cfg}
}

func (ji *HPSearchJobInstance) StartHPSearchJob() error {
	procs, err := hpsearch.StartJob(ji.SigmaClnt, ji.cfg.NConfigs, ji.cfg.Seed, ji.cfg.MaxIters, ji.cfg.IterDur, ji.cfg.Mcpu)
	if err != nil {
		return err
	}
	ji.procs = procs
	return nil
}

// StartHPSearchPruningJob starts the same NConfigs synthetic trainers as
// StartHPSearchJob (same seeds, same synthetic curves), but using the
// hp-trainer-pruned variant, which prunes itself online against its
// siblings' live progress instead of always running to completion. Returns
// the shared progress directory so the caller can clean it up afterwards.
// WaitJobExit is reused unmodified for either job type.
func (ji *HPSearchJobInstance) StartHPSearchPruningJob(margin float64) (string, error) {
	procs, progressDir, err := hpsearch.StartPruningJob(ji.SigmaClnt, ji.cfg.NConfigs, ji.cfg.Seed, ji.cfg.MaxIters, ji.cfg.IterDur, ji.cfg.Mcpu, margin)
	if err != nil {
		return "", err
	}
	ji.procs = procs
	return progressDir, nil
}

// WaitJobExit waits for every trainer proc to exit and returns each config's
// resulting learning curve.
func (ji *HPSearchJobInstance) WaitJobExit() ([]*hpsearch.Curve, error) {
	curves := make([]*hpsearch.Curve, len(ji.procs))
	for i, p := range ji.procs {
		// Block until this config's trainer exits.
		status, err := ji.WaitExit(p.GetPid())
		if err != nil {
			return nil, err
		}
		// A non-OK status means the trainer crashed rather than reporting a
		// curve; surface that instead of silently decoding a zero-valued Curve.
		if !status.IsStatusOK() {
			return nil, fmt.Errorf("hp-trainer %v exited with non-OK status %v: %v", p.GetPid(), status.StatusCode, status.Msg())
		}
		// Decode the Curve back out of the generic status data.
		curve, err := hpsearch.NewCurve(status.Data())
		if err != nil {
			return nil, err
		}
		curves[i] = curve
	}
	return curves, nil
}
