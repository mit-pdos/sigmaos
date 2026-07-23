package hpsearch

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	TrainerBin        = "hp-trainer"
	PruningTrainerBin = "hp-trainer-pruned"

	// ProgressDirTop is where StartPruningJob creates each job's shared,
	// per-config progress directory (see pruner.go).
	ProgressDirTop = sp.NAMED + "hpsearch-progress/"

	// PruneMargin is how far behind the best config's score a config must
	// fall (and stay) before it is pruned -- by the live policy in
	// pruner.go, and by the post-hoc oracle in analysis.go alike.
	PruneMargin = 0.1
)

// Config is one hyperparameter search: how many configs to try, how long to
// run each, and what resources to give them.
type Config struct {
	NConfigs int
	MaxIters int
	IterDur  time.Duration
	Mcpu     proc.Tmcpu
	Seed     int64
	Margin   float64
}

func DefaultConfig() *Config {
	return &Config{
		NConfigs: 50,
		MaxIters: 20,
		IterDur:  50 * time.Millisecond,
		Mcpu:     1000,
		Seed:     7159623, // Chosen to make the synthetic curves reproducible
		Margin:   PruneMargin,
	}
}

// Job is a search whose trainers have been spawned but not yet reaped, so a
// caller can start the job with StartJob, add contention to the cluster
// while it runs, and only then block on the results with Wait. Wait tears
// down whatever shared state the job created, so callers never clean up
// after it.
type Job struct {
	sc    *sigmaclnt.SigmaClnt
	cfg   *Config
	procs []*proc.Proc
	// progressDir is where the pruning trainers publish their scores; empty
	// for a baseline job, which shares nothing.
	progressDir string
}

// Procs returns the job's trainer procs, in configId order, so a caller can
// inspect or evict them between StartJob and Wait.
func (j *Job) Procs() []*proc.Proc {
	return j.procs
}

// ProgressDir returns the directory this job's trainers publish scores to,
// or "" for a baseline job. Wait removes it; callers only need it to
// observe a job's pruning decisions while it runs.
func (j *Job) ProgressDir() string {
	return j.progressDir
}

// WaitStart blocks until every trainer is actually running, rather than
// merely spawned; Wait subsumes it. Use it to hold contention back until
// the whole search is scheduled. Note that with NConfigs larger than the
// cluster has cores, later trainers cannot start until earlier ones exit,
// so this can block until the job is nearly over.
func (j *Job) WaitStart() error {
	for _, p := range j.procs {
		if err := j.sc.WaitStart(p.GetPid()); err != nil {
			return err
		}
	}
	return nil
}

// Wait blocks until every trainer has exited and returns each config's
// learning curve, in configId order. It also tears down any state the job
// created, so it must be called exactly once per started Job.
func (j *Job) Wait() ([]*Curve, error) {
	// Clean up the shared progress directory, if this job has one, once
	// every trainer is done with it. Failing to remove it doesn't
	// invalidate the curves, so just log it.
	if j.progressDir != "" {
		defer func() {
			if err := j.sc.RmDir(j.progressDir); err != nil {
				db.DPrintf(db.HPSEARCH, "Job.Wait: RmDir %v err %v", j.progressDir, err)
			}
		}()
	}
	return WaitJobExit(j.sc, j.procs)
}

// SpawnTrainer spawns a single hp-trainer proc for one hyperparameter
// configuration, without waiting for it to start running.
func SpawnTrainer(sc *sigmaclnt.SigmaClnt, configId int, seed int64, maxIters int, iterDur time.Duration, mcpu proc.Tmcpu) (*proc.Proc, error) {
	args := []string{
		strconv.Itoa(configId),
		strconv.FormatInt(seed, 10),
		strconv.Itoa(maxIters),
		strconv.FormatInt(iterDur.Milliseconds(), 10),
	}

	p := proc.NewProc(TrainerBin, args)
	p.SetMcpu(mcpu)

	if err := sc.Spawn(p); err != nil {
		return nil, err
	}
	return p, nil
}

// StartTrainer spawns a single hp-trainer proc and waits for it to start
// running.
func StartTrainer(sc *sigmaclnt.SigmaClnt, configId int, seed int64, maxIters int, iterDur time.Duration, mcpu proc.Tmcpu) (*proc.Proc, error) {
	p, err := SpawnTrainer(sc, configId, seed, maxIters, iterDur, mcpu)
	if err != nil {
		return nil, err
	}
	if err := sc.WaitStart(p.GetPid()); err != nil {
		return nil, err
	}
	return p, nil
}

// StartJob spawns cfg.NConfigs independent hp-trainer procs, one per
// hyperparameter configuration (configId 0..NConfigs-1), each seeded off
// cfg.Seed so runs are reproducible. There is no coordinator process and no
// pruning: every config runs to completion, which is exactly the baseline
// behavior this benchmark measures.
//
// It returns as soon as every trainer is spawned, without waiting for them
// to be scheduled, so the caller gets control back while the search is
// still running. Call Job.Wait for the curves.
func StartJob(sc *sigmaclnt.SigmaClnt, cfg *Config) (*Job, error) {
	// Random number generator for seeds
	rng := rand.New(rand.NewSource(cfg.Seed))

	procs := make([]*proc.Proc, cfg.NConfigs)
	// Spawn one trainer per config, each with its own derived seed.
	for i := 0; i < cfg.NConfigs; i++ {
		p, err := SpawnTrainer(sc, i, rng.Int63(), cfg.MaxIters, cfg.IterDur, cfg.Mcpu)
		if err != nil {
			return nil, err
		}
		procs[i] = p
	}
	return &Job{sc: sc, cfg: cfg, procs: procs}, nil
}

// SpawnPruningTrainer spawns a single hp-trainer-pruned proc for one
// hyperparameter configuration, without waiting for it to start running.
// Unlike SpawnTrainer, it also passes a shared progressDir and margin so
// the trainer can prune itself against its siblings' live progress (see
// RunPruningTrainer in pruner.go).
func SpawnPruningTrainer(sc *sigmaclnt.SigmaClnt, configId int, seed int64, maxIters int, iterDur time.Duration, mcpu proc.Tmcpu, progressDir string, margin float64) (*proc.Proc, error) {
	// Same argv as SpawnTrainer, plus the progressDir/margin the trainer
	// needs to prune itself against its siblings.
	args := []string{
		strconv.Itoa(configId),
		strconv.FormatInt(seed, 10),
		strconv.Itoa(maxIters),
		strconv.FormatInt(iterDur.Milliseconds(), 10),
		progressDir,
		strconv.FormatFloat(margin, 'f', -1, 64),
	}
	p := proc.NewProc(PruningTrainerBin, args)
	p.SetMcpu(mcpu)
	if err := sc.Spawn(p); err != nil {
		return nil, err
	}
	return p, nil
}

// StartPruningTrainer spawns a single hp-trainer-pruned proc and waits for
// it to start running.
func StartPruningTrainer(sc *sigmaclnt.SigmaClnt, configId int, seed int64, maxIters int, iterDur time.Duration, mcpu proc.Tmcpu, progressDir string, margin float64) (*proc.Proc, error) {
	p, err := SpawnPruningTrainer(sc, configId, seed, maxIters, iterDur, mcpu, progressDir, margin)
	if err != nil {
		return nil, err
	}
	if err := sc.WaitStart(p.GetPid()); err != nil {
		return nil, err
	}
	return p, nil
}

// StartPruningJob spawns cfg.NConfigs hp-trainer-pruned procs (configId
// 0..NConfigs-1, seeded exactly like StartJob so the two are comparable),
// sharing one freshly-created progress directory so they can prune
// themselves against each other's live scores instead of always running to
// completion. Like StartJob it returns as soon as the trainers are spawned;
// Job.Wait collects the curves and removes the progress directory.
func StartPruningJob(sc *sigmaclnt.SigmaClnt, cfg *Config) (*Job, error) {
	// Give this run its own uniquely-named progress directory.
	progressDir := ProgressDirTop + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := sc.MkDirPath(sp.NAMED, strings.TrimPrefix(progressDir, sp.NAMED), 0777); err != nil {
		return nil, err
	}

	// Random number generator for seeds
	rng := rand.New(rand.NewSource(cfg.Seed))

	procs := make([]*proc.Proc, cfg.NConfigs)
	// Spawn one pruning trainer per config, all sharing progressDir.
	for i := 0; i < cfg.NConfigs; i++ {
		p, err := SpawnPruningTrainer(sc, i, rng.Int63(), cfg.MaxIters, cfg.IterDur, cfg.Mcpu, progressDir, cfg.Margin)
		if err != nil {
			return nil, err
		}
		procs[i] = p
	}
	return &Job{sc: sc, cfg: cfg, procs: procs, progressDir: progressDir}, nil
}

// WaitJobExit waits for every trainer proc of a job to exit and returns each
// config's resulting learning curve, in the order the procs were spawned.
func WaitJobExit(sc *sigmaclnt.SigmaClnt, procs []*proc.Proc) ([]*Curve, error) {
	curves := make([]*Curve, len(procs))
	for i, p := range procs {
		// Block until this config's trainer exits.
		status, err := sc.WaitExit(p.GetPid())
		if err != nil {
			return nil, err
		}
		// A non-OK status means the trainer crashed rather than reporting a
		// curve; surface that instead of silently decoding a zero-valued Curve.
		if !status.IsStatusOK() {
			return nil, fmt.Errorf("hp-trainer %v exited with non-OK status %v: %v", p.GetPid(), status.StatusCode, status.Msg())
		}
		// Decode the Curve back out of the generic status data.
		curve, err := NewCurve(status.Data())
		if err != nil {
			return nil, err
		}
		curves[i] = curve
	}
	return curves, nil
}
