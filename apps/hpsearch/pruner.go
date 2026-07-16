package hpsearch

import (
	"fmt"
	"path"
	"strconv"
	"time"

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
	sc.Remove(pn)
	if _, err := sc.PutFile(pn, 0777, sp.OWRITE, []byte(fmt.Sprintf("%d %f", iter, score))); err != nil {
		db.DPrintf(db.HPSEARCH, "publishProgress: PutFile %v err %v", pn, err)
	}
}

// bestSiblingScore scans every other config's published progress and
// returns the best (highest) score any sibling has reported so far — i.e.
// exactly what a live, causal policy can see, with no knowledge of how
// those siblings' curves will look in the future (contrast with the
// post-hoc oracle in benchmarks/hpsearch_bench_test.go, which gets to look
// at every config's completed curve).
func bestSiblingScore(sc *fslib.FsLib, progressDir string, selfConfigId int) (best float64, found bool) {
	sts, err := sc.GetDir(progressDir)
	if err != nil {
		return 0, false
	}
	for _, st := range sts {
		cid, err := strconv.Atoi(st.Name)
		if err != nil || cid == selfConfigId {
			continue
		}
		b, err := sc.GetFile(path.Join(progressDir, st.Name))
		if err != nil {
			continue
		}
		var iter int
		var score float64
		if _, err := fmt.Sscanf(string(b), "%d %f", &iter, &score); err != nil {
			continue
		}
		if !found || score > best {
			best = score
			found = true
		}
	}
	return best, found
}

// RunPruningTrainer is the entry point for the hp-trainer-pruned proc. It
// generates the exact same synthetic curve as RunTrainer (same seeded
// syntheticCurve, same Curve result type), but instead of always running to
// completion, it publishes its score after every iteration and prunes
// itself the first time it has trailed the best sibling score seen so far
// by more than margin, for SustainIters iterations in a row. Because this
// decision is made online — with no knowledge of the future — it can (and
// sometimes will) prune a config that would have caught up later, unlike
// the post-hoc oracle.
func RunPruningTrainer(args []string) {
	if len(args) != 6 {
		db.DFatalf("RunPruningTrainer: wrong number of args %v", args)
	}
	configId, seed, maxIters, iterDur, err := parseTrainerArgs(args[:4])
	if err != nil {
		db.DFatalf("RunPruningTrainer: %v", err)
	}
	progressDir := args[4]
	margin, err := strconv.ParseFloat(args[5], 64)
	if err != nil {
		db.DFatalf("RunPruningTrainer: margin %v not a float: %v", args[5], err)
	}

	sc, err := newStartedSigmaClnt()
	if err != nil {
		db.DFatalf("RunPruningTrainer: %v", err)
	}

	asymptote, scores := syntheticCurve(seed, maxIters)

	pruned := false
	prunedAtIter := maxIters
	behindStreak := 0
	for i := range scores {
		time.Sleep(iterDur)
		publishProgress(sc.FsLib, progressDir, configId, i, scores[i])
		db.DPrintf(db.HPSEARCH, "hp-trainer-pruned config %d iter %d score %f", configId, i, scores[i])

		if best, ok := bestSiblingScore(sc.FsLib, progressDir, configId); ok && scores[i] < best-margin {
			behindStreak++
		} else {
			behindStreak = 0
		}
		if behindStreak >= SustainIters {
			pruned = true
			prunedAtIter = i + 1
			scores = scores[:prunedAtIter]
			break
		}
	}

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
