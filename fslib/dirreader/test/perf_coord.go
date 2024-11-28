package test

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sigmaos/fslib/dirreader"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
	"strconv"
	"time"

	db "sigmaos/debug"
)

type Result struct {
	CreationWatchTimeNs [][]time.Duration
	DeletionWatchTimeNs [][]time.Duration
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
	baseDir string
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

	baseDir := args[3]
	watchDir := filepath.Join(baseDir, "watch")
	responseDir := filepath.Join(baseDir, "response")
	tempDir := filepath.Join(baseDir, "temp")
	signalDir := filepath.Join(baseDir, "signal")

	measureMode, err := strconv.Atoi(args[4])
	if err != nil || measureMode < 0 || measureMode > 1 {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: measure mode %s is invalid", args[5])
	}

	return &PerfCoord{
		sc,
		nWorkers,
		nStartFiles,
		nTrials,
		baseDir,
		watchDir,
		responseDir,
		tempDir,
		signalDir,
		MeasureMode(measureMode),
	}, nil
}

func (c *PerfCoord) Run() {
	c.MkDir(c.baseDir, 0777)
	c.MkDir(c.watchDir, 0777)
	c.MkDir(c.responseDir, 0777)
	c.MkDir(c.tempDir, 0777)
	c.MkDir(c.signalDir, 0777)

	for ix := 0; ix < c.nStartFiles; ix++ {
		path := filepath.Join(c.watchDir, strconv.Itoa(ix))
		fd, err := c.Create(path, 0777, sigmap.OAPPEND)
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
			p := proc.NewProc("watchperf-worker", []string{strconv.Itoa(ix), strconv.Itoa(c.nTrials), c.watchDir, c.responseDir, c.tempDir, c.signalDir, strconv.Itoa(int(c.measureMode)), strconv.Itoa(c.nStartFiles)})
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

	creationWatchDelays := make([][]time.Duration, c.nTrials)
	deletionWatchDelays := make([][]time.Duration, c.nTrials)

	for trial := 0; trial < c.nTrials; trial++ {
		db.DPrintf(db.WATCH_PERF, "Run: Running trial %d", trial)
		filename := fmt.Sprintf("trial_%d", trial)
		path := filepath.Join(c.watchDir, filename)
		signal_path_create := filepath.Join(c.signalDir, coordSignalName(trial, false))
		signal_path_delete := filepath.Join(c.signalDir, coordSignalName(trial, true))

		db.DPrintf(db.WATCH_PERF, "Run: Creating file for trial %d", trial)

		var creationStart time.Time
		var deletionStart time.Time

		if c.measureMode == JustWatch {
			creationStart = time.Now()
			_, err = c.Create(path, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create trial file %d, %v", trial, err)
			}
			// wait for us to see it locally before signaling the worker to watch
			err := dirreader.WaitCreate(c.FsLib, path)
			if err != nil {
				db.DFatalf("Run: failed to wait for file creation %v", err)
			}
			time.Sleep(10 * time.Millisecond) 

			_, err = c.Create(signal_path_create, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
			}
		} else {
			_, err = c.Create(signal_path_create, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
			}
			// wait for all workers to signal that they have started watching
			for ix := 0; ix < c.nWorkers; ix++ {
				c.waitForWorkerSignal(trial, strconv.Itoa(ix), false)
			}
			creationStart = time.Now()
			_, err = c.Create(path, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create trial file %d, %v", trial, err)
			}
		}

		// wait for all children to recognize the creation
		db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to see creation for trial %d", trial)
		c.waitResponses(responseDirReader, trial, false)

		creationWatchDelays[trial] = c.getWorkerDelays(creationStart, trial, false)

		db.DPrintf(db.WATCH_PERF, "Run: Removing file for trial %d", trial)
		if c.measureMode == JustWatch {
			deletionStart = time.Now()
			err = c.Remove(path)
			if err != nil {
				db.DFatalf("Run: failed to delete trial file %d, %v", trial, err)
			}
			// wait for us to see it locally before signalling the worker to watch
			err := dirreader.WaitRemove(c.FsLib, path)
			if err != nil {
				db.DFatalf("Run: failed to wait for file creation %v", err)
			}
			time.Sleep(10 * time.Millisecond) 

			_, err = c.Create(signal_path_delete, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
			}
		} else {
			_, err = c.Create(signal_path_delete, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
			}
			// wait for all workers to signal that they have started watching
			for ix := 0; ix < c.nWorkers; ix++ {
				c.waitForWorkerSignal(trial, strconv.Itoa(ix), true)
			}
			deletionStart = time.Now()
			err = c.Remove(path)
			if err != nil {
				db.DFatalf("Run: failed to delete trial file %d, %v", trial, err)
			}
		}
		if err != nil {
			db.DFatalf("Run: failed to remove trial file %d, %v", trial, err)
		}

		// wait for all children to recognize the deletion
		db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to see deletion for trial %d", trial)
		c.waitResponses(responseDirReader, trial, true)

		deletionWatchDelays[trial] = c.getWorkerDelays(deletionStart, trial, true)
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
	if err := c.RmDirEntries(c.signalDir); err != nil {
		db.DFatalf("Run: failed to clear signaldir entries %v", err)
	}

	if err := c.Remove(c.signalDir); err != nil {
		db.DFatalf("Run: failed to remove signaldir %v", err)
	}
	if err := c.Remove(c.baseDir); err != nil {
		db.DFatalf("Run: failed to remove basedir %v", err)
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

func (c *PerfCoord) getWorkerDelays(startTime time.Time, trialNum int, deleted bool) []time.Duration {
	db.DPrintf(db.WATCH_PERF, "getWorkerDelays: Getting proc times")
	times := c.getWorkerTimes(trialNum, deleted)
	db.DPrintf(db.WATCH_PERF, "worker times: %v", times)

	deltas := make([]time.Duration, c.nWorkers)
	for ix := 0; ix < c.nWorkers; ix++ {
		if c.measureMode == JustWatch {
			deltas[ix] = time.Duration(times[ix].Nanosecond())
		} else if c.measureMode == IncludeFileOp {
			deltas[ix] = times[ix].Sub(startTime)
		} else {
			db.DFatalf("getWorkerDelays: invalid measure mode %d", c.measureMode)
		}
	}

	return deltas
}

func (c *PerfCoord) getWorkerTimes(trialNum int, deleted bool) []time.Time {
	times := make([]time.Time, c.nWorkers)
	db.DPrintf(db.WATCH_PERF, "getWorkerTimes: Getting proc times")

	for ix := 0; ix < c.nWorkers; ix++ {
		path := filepath.Join(c.responseDir, responseName(trialNum, strconv.Itoa(ix), deleted))
		reader, err := c.OpenReader(path)
		if err != nil {
			db.DFatalf("getWorkerTimes: failed to open file %s, %v", path, err)
		}
		scanner := bufio.NewScanner(reader)
		exists := scanner.Scan()
		if !exists {
			db.DFatalf("getWorkerTimes: file %s does not contain line", path)
		}

		text := scanner.Text()
		db.DPrintf(db.WATCH_PERF, "getWorkerTimes: File %s has contents %s", path, text)

		if c.measureMode == JustWatch {
			duration, err := strconv.Atoi(text)
			if err != nil {
				db.DFatalf("getWorkerTimes: failed to parse duration %s, %v", text, err)
			}
			times[ix] = time.Unix(0, int64(duration))
		} else {
			time, err := time.Parse(time.RFC3339Nano, text)
			if err != nil {
				db.DFatalf("getWorkerTimes: failed to parse time %s, %v", text, err)
			}
			times[ix] = time
		}
	}

	return times
}

func (c *PerfCoord) waitResponses(responseDirReader dirreader.DirReader, trialNum int, deleted bool) {
	numFiles := trialNum * 2 + 2 // 1 for the initial responses, 2 for each prev trial, 1 for the current trial
	if deleted {
		numFiles += 1 // 1 for the creation ones
	}
	numFiles *= c.nWorkers

	err := responseDirReader.WaitNEntries(numFiles)
	if err != nil {
		db.DFatalf("Run: failed to wait for all procs to respond %v", err)
	}
}


func (c *PerfCoord) waitForWorkerSignal(trial int, workerId string, delete bool) {
	signalPath := filepath.Join(c.signalDir, workerSignalName(trial, delete, workerId))
	db.DPrintf(db.WATCH_PERF, "waitForWorkerSignal: Waiting for signal %s", signalPath)
	err := dirreader.WaitCreate(c.FsLib, signalPath)
	if err != nil {
		db.DFatalf("waitForWorkerSignal: failed to wait for signal %s, %v", signalPath, err)
	}
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
