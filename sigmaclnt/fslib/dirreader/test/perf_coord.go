package test

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib/dirreader"
	"strconv"
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type Result struct {
	CreationWatchTimeNs [][][]time.Duration
	DeletionWatchTimeNs [][][]time.Duration
}

type MeasureMode int

const (
  // measure time from before file creation / deletion to time the worker notices the file.
	IncludeFileOp MeasureMode = iota

	// measure time from when the worker starts watching to when it notices the file.
	// in this mode, the watch is guaranteed to start after the file is created to measure
	// just the latency of the watch.
	JustWatch     MeasureMode = iota
)

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
	measureMode MeasureMode
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

	measureMode, err := strconv.Atoi(args[5])
	if err != nil || measureMode < 0 || measureMode > 1 {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: measure mode %s is invalid", args[5])
	}

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
		MeasureMode(measureMode),
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
			p := proc.NewProc("watchperf-worker", []string{strconv.Itoa(ix), strconv.Itoa(c.nTrials), strconv.Itoa(c.nFilesPerTrial), c.watchDir, c.responseDir, c.tempDir, c.signalDir, strconv.Itoa(int(c.measureMode)), strconv.Itoa(c.nStartFiles)})
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
	responseDirReader, err := dirreader.NewDirReader(c.FsLib, c.responseDir)
	if err != nil {
		db.DFatalf("Run: failed to create dir watcher for response dir %v", err)
	}
	err = responseDirReader.WaitNEntries(c.nWorkers)
	if err != nil {
		db.DFatalf("Run: failed to wait for all procs to be ready %v", err)
	}

	creationWatchDelays := make([][][]time.Duration, c.nTrials)
	deletionWatchDelays := make([][][]time.Duration, c.nTrials)

	for trial := 0; trial < c.nTrials; trial++ {
		db.DPrintf(db.WATCH_PERF, "Run: Running trial %d", trial)
		creationWatchDelays[trial] = c.handleTrial(trial, false, responseDirReader)
		deletionWatchDelays[trial] = c.handleTrial(trial, true, responseDirReader)
		err := c.RmDirEntries(c.signalDir)
		if err != nil {
			db.DFatalf("Run: failed to clear signaldir entries %v", err)
		}
	}

	err = responseDirReader.Close()
	if err != nil {
		db.DFatalf("RunCoord: failed to close watcher %v", err)
	}

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
		DeletionWatchTimeNs: deletionWatchDelays,
	}
	status := proc.NewStatusInfo(proc.StatusOK, "", result)
	err = c.ClntExit(status)
	if err != nil {
		db.DFatalf("Run: failed to exit client %v", err)
	}
}

func (c *PerfCoord) handleTrial(trial int, delete bool, responseDirReader *dirreader.DirReader) [][]time.Duration {
	signalPath := filepath.Join(c.signalDir, coordSignalName(trial, delete))

	opType := "create"
	if delete {
		opType = "delete"
	}

	db.DPrintf(db.WATCH_PERF, "Run: %s file for trial %d (measure mode = %d)", opType, trial, c.measureMode)

	var opStart time.Time
	var err error

	if c.measureMode == JustWatch {
		for ix := 0; ix < c.nFilesPerTrial; ix++ {
			path := filepath.Join(c.watchDir, trialName(trial, ix))
			opStart = time.Now()
			if !delete {
				_, err = c.Create(path, 0777, sp.OAPPEND)
			} else {
				err = c.Remove(path)
			}
			if err != nil {
				db.DFatalf("Run: failed to %s trial file %d %d, %v", opType, trial, ix, err)
			}

			// wait for us to see it locally before signaling the worker to watch
			if !delete {
				err = dirreader.WaitCreate(c.FsLib, path)
			} else {
				err = dirreader.WaitRemove(c.FsLib, path)
			}
			if err != nil {
				db.DFatalf("Run: failed to wait for file creation %v", err)
			}
		}
		time.Sleep(10 * time.Millisecond) 

		_, err = c.Create(signalPath, 0777, sp.OAPPEND)
		if err != nil {
			db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
		}
	} else {
		_, err = c.Create(signalPath, 0777, sp.OAPPEND)
		if err != nil {
			db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
		}
		db.DPrintf(db.WATCH_PERF, "Run: waiting for worker signals for trial %d", trial)
		c.waitForWorkerSignals(trial, delete)
		opStart = time.Now()
		db.DPrintf(db.WATCH_PERF, "Run: received worker signals, now starting the goroutines for deleting in trial %d", trial)
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
	}

	// wait for all children to recognize the op
	db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to see %s for trial %d", opType, trial)
	c.waitResponses(responseDirReader, trial, delete)

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
			if c.measureMode == JustWatch {
				deltas[ix][jx] = time.Duration(times[ix][jx].Nanosecond())
			} else if c.measureMode == IncludeFileOp {
				deltas[ix][jx] = times[ix][jx].Sub(startTime)
			} else {
				db.DFatalf("getWorkerDelays: invalid measure mode %d", c.measureMode)
			}
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

			if c.measureMode == JustWatch {
				duration, err := strconv.Atoi(text)
				if err != nil {
					db.DFatalf("getWorkerTimes: failed to parse duration %s, %v", text, err)
				}
				times[ix][jx] = time.Unix(0, int64(duration))
			} else {
				time, err := time.Parse(time.RFC3339Nano, text)
				if err != nil {
					db.DFatalf("getWorkerTimes: failed to parse time %s, %v", text, err)
				}
				times[ix][jx] = time
			}
		}
	}

	return times
}

func (c *PerfCoord) waitResponses(responseDirReader *dirreader.DirReader, trialNum int, deleted bool) {
	for ix := 0; ix < c.nWorkers; ix++ {
		err := responseDirReader.WaitCreate(responseName(trialNum, strconv.Itoa(ix), deleted))
		if err != nil {
			db.DFatalf("Run: failed to wait for all procs to respond %v", err)
		}
	}
}

func (c *PerfCoord) waitForWorkerSignals(trial int, delete bool) {
	for ix := 0; ix < c.nWorkers; ix++ {
		workerId := strconv.Itoa(ix)
		signalPath := filepath.Join(c.signalDir, workerSignalName(trial, delete, workerId))
		db.DPrintf(db.WATCH_PERF, "waitForWorkerSignal: Waiting for signal %s", signalPath)
		err := dirreader.WaitCreate(c.FsLib, signalPath)
		if err != nil {
			db.DFatalf("waitForWorkerSignal: failed to wait for signal %s, %v", signalPath, err)
		}
	}
}

func trialName(trial int, ix int) string {
	return fmt.Sprintf("trial_%d_%d", trial, ix)
}

func coordSignalName(trial int, delete bool) string {
	if delete {
		return fmt.Sprintf("coord_signal_delete_%d", trial)
	}
	return fmt.Sprintf("coord_signal_create_%d", trial)
}

func workerSignalName(trial int, delete bool, workerId string) string {
	if delete {
		return fmt.Sprintf("worker_%s_signal_delete_%d", workerId, trial)
	}
	return fmt.Sprintf("worker_%s_signal_create_%d", workerId, trial)
}

func responseName(trialNum int, workerNum string, deleted bool) string {
	if deleted {
		return fmt.Sprintf("trial_%d_worker_%s_deleted", trialNum, workerNum)
	}
	return fmt.Sprintf("trial_%d_worker_%s_created", trialNum, workerNum)
}
