package test

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib/dircache"
	"strconv"
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type Result struct {
	CreationWatchTimeNs [][][]time.Duration
}

type PerfCoord struct {
	*sigmaclnt.SigmaClnt
  nWorkers int
	nStartFiles int
	nTrials int
	nFilesPerTrial int
	namedBase string
	localBase string
	watchDir string
	responseDir string
	tempDir string
	signalDir string
}

func NewPerfCoord(args []string) (*PerfCoord, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: error %v", err)
	}

	err = sc.Started()
	if err != nil {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: error %v", err)
	}

	nWorkers, err := strconv.Atoi(args[0])
	if err != nil {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: nworkers %s is not an integer", args[0])
	}

	nStartFiles, err := strconv.Atoi(args[1])
	if err != nil {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: nstartfiles %s is not an integer", args[1])
	}

	nTrials, err := strconv.Atoi(args[2])
	if err != nil {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: nTrials %s is not an integer", args[2])
	}

	nFilesPerTrial, err := strconv.Atoi(args[3])
	if err != nil {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: nFilesPerTrial %s is not an integer", args[3])
	}

	useNamed := args[4] == "1"

	namedBase := filepath.Join(sp.NAMED, "watchperf")
	localBase := filepath.Join(sp.UX, sp.LOCAL, "watchperf")
	
	var watchDir string
	if useNamed {
		watchDir = filepath.Join(namedBase, "watch")
	} else {
		watchDir = filepath.Join(localBase, "watch")
	}
	responseDir := filepath.Join(localBase, "response")
	tempDir := filepath.Join(localBase, "temp")
	signalDir := filepath.Join(localBase, "signal")

	return &PerfCoord{
		sc,
		nWorkers,
		nStartFiles,
		nTrials,
		nFilesPerTrial,
		namedBase,
		localBase,
		watchDir,
		responseDir,
		tempDir,
		signalDir,
	}, nil
}

func (c *PerfCoord) Run() {
	c.MkDir(c.namedBase, 0777)
	c.MkDir(c.localBase, 0777)
	c.MkDir(c.watchDir, 0777)
	c.MkDir(c.responseDir, 0777)
	c.MkDir(c.tempDir, 0777)
	c.MkDir(c.signalDir, 0777)

	for ix := 0; ix < c.nStartFiles; ix++ {
		path := filepath.Join(c.watchDir, strconv.Itoa(ix))
		fd, err := c.Create(path, 0777, sp.OAPPEND)
		if err != nil {
			db.DFatalf("Run: failed to create starting file %d, %v", ix, err)
		}
		err = c.CloseFd(fd)
		if err != nil {
			db.DFatalf("Run: failed to close fd for starting file %d, %v", ix, err)
		}
	}

	for ix := 0; ix < c.nWorkers; ix++ {
		go func (ix int) {
			p := proc.NewProc("watchperf-worker", []string{strconv.Itoa(ix), strconv.Itoa(c.nTrials), strconv.Itoa(c.nFilesPerTrial), c.watchDir, c.responseDir, c.tempDir, c.signalDir, strconv.Itoa(c.nStartFiles)})
			err := c.Spawn(p)
			if err != nil {
				db.DFatalf("Run: spawning %d failed %v", ix, err)
			}
			err = c.WaitStart(p.GetPid())
			if err != nil {
				db.DFatalf("Run: starting %d failed %v", ix, err)
			}
			_, err = c.WaitExit(p.GetPid())
			if err != nil {
				db.DFatalf("Run: running %d failed %v", ix, err)
			}
		}(ix)
	}

	db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to signal they're ready")
	responseDirCache := dircache.NewDirCache(c.FsLib, c.responseDir, func(_ string) (struct{}, error) {
		return struct{}{}, nil
	}, nil, db.WATCH_PERF, db.WATCH_PERF)
	_, err := responseDirCache.WaitEntriesN(c.nWorkers, true)
	if err != nil {
		db.DFatalf("Run: failed to wait for all procs to be ready %v", err)
	}

	creationWatchDelays := make([][][]time.Duration, c.nTrials)

	for trial := 0; trial < c.nTrials; trial++ {
		db.DPrintf(db.WATCH_PERF, "Run: Running trial %d", trial)
		creationWatchDelays[trial] = c.handleTrial(trial, false, responseDirCache)
		c.handleTrial(trial, true, responseDirCache)
		err := c.RmDirEntries(c.signalDir)
		if err != nil {
			db.DFatalf("Run: failed to clear signaldir entries %v", err)
		}
	}

	responseDirCache.StopWatching()

	if err := c.RmDirEntries(c.responseDir); err != nil {
		db.DFatalf("Run: failed to clear response dir entries %v", err)
	}

	if err := c.RmDirEntries(c.watchDir); err != nil {
		db.DFatalf("Run: failed to clear watchdir entries %v", err)
	}
	if err := c.Remove(c.watchDir); err != nil {
		db.DFatalf("Run: failed to remove watchdir %v", err)
	}

	if err := c.Remove(c.responseDir); err != nil {
		db.DFatalf("Run: failed to remove responsedir %v", err)
	}
	if err := c.Remove(c.tempDir); err != nil {
		db.DFatalf("Run: failed to remove tempdir %v", err)
	}
	if err := c.Remove(c.signalDir); err != nil {
		db.DFatalf("Run: failed to remove signaldir %v", err)
	}

	if err := c.Remove(c.namedBase); err != nil {
		db.DFatalf("Run: failed to remove named base %v", err)
	}
	if err := c.Remove(c.localBase); err != nil {
		db.DFatalf("Run: failed to remove local base %v", err)
	}

	result := Result{
		CreationWatchTimeNs: creationWatchDelays,
	}
	status := proc.NewStatusInfo(proc.StatusOK, "", result)
	err = c.ClntExit(status)
	if err != nil {
		db.DFatalf("Run: failed to exit client %v", err)
	}
}

func (c *PerfCoord) handleTrial(trial int, delete bool, responseDirCache *dircache.DirCache[struct{}]) [][]time.Duration {
	opType := "create"
	if delete {
		opType = "delete"
	}

	db.DPrintf(db.WATCH_PERF, "Run: %s file for trial %d", opType, trial)

	opStart := time.Now()
	db.DPrintf(db.WATCH_PERF, "Run: starting the goroutines for %s in trial %d", opType, trial)
	for ix := 0; ix < c.nFilesPerTrial; ix++ {
		go func (ix int) {
			var err error
			path := filepath.Join(c.watchDir, trialName(trial, ix))
			if !delete {
				_, err = c.Create(path, 0777, sp.OAPPEND)
			} else {
				err = c.Remove(path)
			}
			if err != nil {
				db.DFatalf("Run: failed to %s trial file %d %d, %v", opType, trial, ix, err)
			}
		}(ix)
	}

	// wait for all children to recognize the op
	db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to see %s for trial %d", opType, trial)
	c.waitResponses(responseDirCache, trial, delete)
	return c.getWorkerDelays(opStart, trial, delete)
}

func (c *PerfCoord) getWorkerDelays(startTime time.Time, trialNum int, deleted bool) [][]time.Duration {
	db.DPrintf(db.WATCH_PERF, "getWorkerDelays: Getting proc times")
	times := c.getWorkerTimes(trialNum, deleted)
	db.DPrintf(db.WATCH_PERF, "start time: %v, worker times: %v", startTime, times)

	deltas := make([][]time.Duration, c.nWorkers)
	for ix := 0; ix < c.nWorkers; ix++ {
		deltas[ix] = make([]time.Duration, c.nFilesPerTrial)
		for jx := 0; jx < c.nFilesPerTrial; jx++ {
			deltas[ix][jx] = times[ix][jx].Sub(startTime)
		}
	}

	return deltas
}

func (c *PerfCoord) getWorkerTimes(trialNum int, deleted bool) [][]time.Time {
	times := make([][]time.Time, c.nWorkers)
	db.DPrintf(db.WATCH_PERF, "getWorkerTimes: Getting proc times")

	for ix := 0; ix < c.nWorkers; ix++ {
		times[ix] = make([]time.Time, c.nFilesPerTrial)
		path := filepath.Join(c.responseDir, responseName(trialNum, strconv.Itoa(ix), deleted))
		reader, err := c.OpenReader(path)
		if err != nil {
			db.DFatalf("getWorkerTimes: failed to open file %s, %v", path, err)
		}
		scanner := bufio.NewScanner(reader)

		for jx := 0; jx < c.nFilesPerTrial; jx++ {
			exists := scanner.Scan()
			if !exists {
				db.DFatalf("getWorkerTimes: file %s does not contain line %d", path, jx + 1)
			}
			text := scanner.Text()

			time, err := time.Parse(time.RFC3339Nano, text)
			if err != nil {
				db.DFatalf("getWorkerTimes: failed to parse time %s, %v", text, err)
			}
			times[ix][jx] = time
		}
	}

	return times
}

func (c *PerfCoord) waitResponses(responseDirCache *dircache.DirCache[struct{}], trialNum int, deleted bool) {
	for ix := 0; ix < c.nWorkers; ix++ {
		err := responseDirCache.WaitEntryCreated(responseName(trialNum, strconv.Itoa(ix), deleted))
		if err != nil {
			db.DFatalf("Run: failed to wait for all procs to respond %v", err)
		}
	}
}

func trialName(trial int, ix int) string {
	return fmt.Sprintf("trial_%d_%d", trial, ix)
}

func responseName(trialNum int, workerNum string, deleted bool) string {
	if deleted {
		return fmt.Sprintf("trial_%d_worker_%s_deleted", trialNum, workerNum)
	}
	return fmt.Sprintf("trial_%d_worker_%s_created", trialNum, workerNum)
}
