package hpsearch

import (
	"strconv"
	"strings"
	"time"

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
)

// StartTrainer spawns a single hp-trainer proc for one hyperparameter
// configuration and waits for it to start running.
func StartTrainer(sc *sigmaclnt.SigmaClnt, configId int, seed int64, maxIters int, iterDur time.Duration, mcpu proc.Tmcpu) (*proc.Proc, error) {
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
	if err := sc.WaitStart(p.GetPid()); err != nil {
		return nil, err
	}
	return p, nil
}

// StartJob spawns nconfigs independent hp-trainer procs, one per
// hyperparameter configuration (configId 0..nconfigs-1), each seeded off
// baseSeed so runs are reproducible. There is no coordinator process and no
// pruning: every config runs to completion, which is exactly the baseline
// behavior this benchmark measures.
func StartJob(sc *sigmaclnt.SigmaClnt, nconfigs int, baseSeed int64, maxIters int, iterDur time.Duration, mcpu proc.Tmcpu) ([]*proc.Proc, error) {
	procs := make([]*proc.Proc, nconfigs)
	for i := 0; i < nconfigs; i++ {
		p, err := StartTrainer(sc, i, baseSeed+int64(i), maxIters, iterDur, mcpu)
		if err != nil {
			return nil, err
		}
		procs[i] = p
	}
	return procs, nil
}

// StartPruningTrainer spawns a single hp-trainer-pruned proc for one
// hyperparameter configuration. Unlike StartTrainer, it also passes a
// shared progressDir and margin so the trainer can prune itself against
// its siblings' live progress (see RunPruningTrainer in pruner.go).
func StartPruningTrainer(sc *sigmaclnt.SigmaClnt, configId int, seed int64, maxIters int, iterDur time.Duration, mcpu proc.Tmcpu, progressDir string, margin float64) (*proc.Proc, error) {
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
	if err := sc.WaitStart(p.GetPid()); err != nil {
		return nil, err
	}
	return p, nil
}

// StartPruningJob spawns nconfigs hp-trainer-pruned procs (configId
// 0..nconfigs-1, seeded exactly like StartJob so the two are comparable),
// sharing one freshly-created progress directory so they can prune
// themselves against each other's live scores instead of always running to
// completion. Returns the progress directory alongside the procs so the
// caller can clean it up once the job is done.
func StartPruningJob(sc *sigmaclnt.SigmaClnt, nconfigs int, baseSeed int64, maxIters int, iterDur time.Duration, mcpu proc.Tmcpu, margin float64) ([]*proc.Proc, string, error) {
	progressDir := ProgressDirTop + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := sc.MkDirPath(sp.NAMED, strings.TrimPrefix(progressDir, sp.NAMED), 0777); err != nil {
		return nil, "", err
	}
	procs := make([]*proc.Proc, nconfigs)
	for i := 0; i < nconfigs; i++ {
		p, err := StartPruningTrainer(sc, i, baseSeed+int64(i), maxIters, iterDur, mcpu, progressDir, margin)
		if err != nil {
			return nil, "", err
		}
		procs[i] = p
	}
	return procs, progressDir, nil
}
