package hpsearch

import (
	"strconv"
	"time"

	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

const TrainerBin = "hp-trainer"

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
