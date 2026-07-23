package hpsearch

import (
	"fmt"
	"path"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

// SustainIters is how many consecutive iterations a config must trail the
// best sibling score by more than the margin before RunPruningTrainer prunes
// itself. It absorbs noise in the synthetic scores so a single bad iteration
// doesn't trigger a prune.
const SustainIters = 3

// progressPath returns the path a config publishes its latest score to, so
// sibling trainers sharing the same progressDir can read it.
func progressPath(progressDir string, configId int) string {
	return path.Join(progressDir, strconv.Itoa(configId))
}

// publishProgress overwrites this config's progress file with its latest
// iteration/score. Best-effort: a failed publish just means siblings won't
// see this iteration's score, not a fatal error for the trainer itself.
func publishProgress(sc *fslib.FsLib, progressDir string, configId, iter int, score float64) {
	pn := progressPath(progressDir, configId)
	// Clear any previous iteration's file before writing the new one.
	sc.Remove(pn)
	if _, err := sc.PutFile(pn, 0777, sp.OWRITE, []byte(fmt.Sprintf("%d %f", iter, score))); err != nil {
		db.DPrintf(db.HPSEARCH, "publishProgress: PutFile %v err %v", pn, err)
	}
}

// bestSiblingScore scans every other config's published progress and
// returns the best (highest) score any sibling has reported so far: what a
// live, causal policy can see, with no knowledge of how those siblings'
// curves develop later. Contrast OraclePruneIters (analysis.go), which
// looks at every config's completed curve.
func bestSiblingScore(sc *fslib.FsLib, progressDir string, selfConfigId int) (best float64, found bool) {
	// List every config that has published progress so far.
	sts, err := sc.GetDir(progressDir)
	if err != nil {
		return 0, false
	}
	for _, st := range sts {
		// Skip our own progress file and anything not named after a configId.
		cid, err := strconv.Atoi(st.Name)
		if err != nil || cid == selfConfigId {
			continue
		}
		// Read that sibling's latest published "<iter> <score>".
		b, err := sc.GetFile(path.Join(progressDir, st.Name))
		if err != nil {
			continue
		}
		var iter int
		var score float64
		if _, err := fmt.Sscanf(string(b), "%d %f", &iter, &score); err != nil {
			continue
		}
		// Track the best score seen across all siblings.
		if !found || score > best {
			best = score
			found = true
		}
	}
	return best, found
}

// RunPruningTrainer is the entry point for the hp-trainer-pruned proc. It
// generates the same synthetic curve as RunTrainer (same seeded
// syntheticCurve, same Curve result type), but instead of always running to
// completion, it publishes its score after every iteration and prunes
// itself once it has trailed the best sibling score by more than margin for
// SustainIters iterations in a row. That decision is online, with no
// knowledge of the future, so it can prune a config that would have caught
// up later, unlike the post-hoc oracle.
func RunPruningTrainer(args []string) {
	if len(args) != 6 {
		db.DFatalf("RunPruningTrainer: wrong number of args %v", args)
	}
	// Args 0-3 are shared with RunTrainer; 4-5 are pruning-specific.
	configId, seed, maxIters, iterDur, err := parseTrainerArgs(args[:4])
	if err != nil {
		db.DFatalf("RunPruningTrainer: %v", err)
	}
	progressDir := args[4]
	margin, err := strconv.ParseFloat(args[5], 64)
	if err != nil {
		db.DFatalf("RunPruningTrainer: margin %v not a float: %v", args[5], err)
	}
	// Log the whole configuration of this run upfront, including the pruning
	// policy's parameters, so a per-iteration log line (or a prune decision)
	// can be traced back to the config that produced it.
	db.DPrintf(db.HPSEARCH, "hp-trainer-pruned start config %d seed %d maxIters %d iterDur %v progressDir %v margin %f sustainIters %d args %v", configId, seed, maxIters, iterDur, progressDir, margin, SustainIters, args)

	// Connect to SigmaOS and signal that this proc has started running.
	sc, err := newStartedSigmaClnt()
	if err != nil {
		db.DFatalf("RunPruningTrainer: %v", err)
	}

	// Same deterministic synthetic curve as the baseline trainer.
	asymptote, scores := syntheticCurve(seed, maxIters)

	pruned := false
	prunedAtIter := maxIters
	behindStreak := 0
	for i := range scores {
		// Simulate one iteration of training, then publish the score.
		SleepBurn(iterDur)
		publishProgress(sc.FsLib, progressDir, configId, i, scores[i])

		// Track how many iterations in a row we've trailed the best sibling.
		if best, ok := bestSiblingScore(sc.FsLib, progressDir, configId); ok && scores[i] < best-margin {
			behindStreak++
		} else {
			behindStreak = 0
		}
		// Once behind for long enough, stop early and report a partial curve.
		if behindStreak >= SustainIters {
			pruned = true
			prunedAtIter = i + 1
			scores = scores[:prunedAtIter]
			break
		}
	}

	// Report the (possibly partial) curve back to whoever is waiting on us.
	curve := Curve{
		ConfigId:     configId,
		Seed:         seed,
		Asymptote:    asymptote,
		Scores:       scores,
		Pruned:       pruned,
		PrunedAtIter: prunedAtIter,
	}
	sc.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK", curve))
}
