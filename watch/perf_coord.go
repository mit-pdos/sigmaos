package watch

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sigmaos/fslib"
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
	IncludeFileOp MeasureMode = iota
	JustWatch                 = iota
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
	useOldWatch bool
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

	var useOldWatch bool
	oldOrNew := args[4]
	if oldOrNew == "old" {
		useOldWatch = true
	} else if (oldOrNew == "new") {
		useOldWatch = false
	} else {
		return &PerfCoord{}, fmt.Errorf("NewPerfCoord: oldornew %s is not either 'old' or 'new'", args[4])
	}

	measureMode, err := strconv.Atoi(args[5])
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
		useOldWatch,
		MeasureMode(measureMode),
	}, nil
}

func (c *PerfCoord) Run() {
	c.MkDir(c.baseDir, 0777)
	c.MkDir(c.watchDir, 0777)
	c.MkDir(c.responseDir, 0777)
	c.MkDir(c.tempDir, 0777)

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
		oldOrNew := "new"
		if c.useOldWatch {
			oldOrNew = "old"
		}

		go func (ix int) {
			p := proc.NewProc("watchperf-worker", []string{strconv.Itoa(ix), strconv.Itoa(c.nTrials), c.watchDir, c.responseDir, c.tempDir, oldOrNew, strconv.Itoa(int(c.measureMode))})
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
	var responseWatcher *fslib.DirWatcher

	if c.useOldWatch {
		dirReader := fslib.NewDirReader(c.FsLib, c.responseDir)
		err := dirReader.WaitNEntries(c.nWorkers)
		if err != nil {
			db.DFatalf("Run: failed to wait for all procs to be ready %v", err)
		}
	} else {
		var err error
		responseWatcher, _, err = fslib.NewDirWatcher(c.FsLib, c.responseDir)
		if err != nil {
			db.DFatalf("Run: failed to create dir watcher for response dir %v", err)
		}
		err = responseWatcher.WaitNEntries(c.nWorkers)
		if err != nil {
			db.DFatalf("RunCoord: failed to wait for all procs to be ready %v", err)
		}
	}
	c.clearResponseDir()

	responseDirFd, err := c.Open(c.responseDir, sigmap.OREAD)
	if err != nil {
		db.DFatalf("Run: failed to open response dir %v", err)
	}

	creationWatchDelays := make([][]time.Duration, c.nTrials)
	deletionWatchDelays := make([][]time.Duration, c.nTrials)

	for trial := 0; trial < c.nTrials; trial++ {
		db.DPrintf(db.WATCH_PERF, "Run: Running trial %d", trial)
		path := filepath.Join(c.watchDir, fmt.Sprintf("trial_%d", trial))
		signal_path_create := filepath.Join(c.watchDir, coordSignalName(trial, false))
		signal_path_delete := filepath.Join(c.watchDir, coordSignalName(trial, true))

		db.DPrintf(db.WATCH_PERF, "Run: Creating file for trial %d", trial)

		var creationStart time.Time
		var deletionStart time.Time

		if c.measureMode == JustWatch {
			creationStart = time.Now()
			_, err = c.Create(path, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create trial file %d, %v", trial, err)
			}
			_, err = c.Create(signal_path_create, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
			}
		} else {
			_, err = c.Create(signal_path_create, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
			}
			creationStart = time.Now()
			_, err = c.Create(path, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create trial file %d, %v", trial, err)
			}
		}

		// wait for all children to recognize the creation
		db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to see creation for trial %d", trial)
		c.waitResponses(responseWatcher)

		creationWatchDelays[trial] = c.getWorkerDelays(creationStart)
		c.clearResponseDir()
		c.waitResponseEmpty(responseDirFd, responseWatcher)

		db.DPrintf(db.WATCH_PERF, "Run: Removing file for trial %d", trial)
		if c.measureMode == JustWatch {
			deletionStart = time.Now()
			err = c.Remove(path)
			if err != nil {
				db.DFatalf("Run: failed to delete trial file %d, %v", trial, err)
			}
			_, err = c.Create(signal_path_delete, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
			}
		} else {
			_, err = c.Create(signal_path_delete, 0777, sigmap.OAPPEND)
			if err != nil {
				db.DFatalf("Run: failed to create signal file %d, %v", trial, err)
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
		c.waitResponses(responseWatcher)

		deletionWatchDelays[trial] = c.getWorkerDelays(deletionStart)
		c.clearResponseDir()
		c.waitResponseEmpty(responseDirFd, responseWatcher)

		err = c.RmDirEntries(c.watchDir)
		if err != nil {
			db.DFatalf("Run: failed to clear watchdir entries %v", err)
		}
	}

	 if !c.useOldWatch {
		if err := responseWatcher.Close(); err != nil {
			db.DFatalf("RunCoord: failed to close watcher %v", err)
		}
	}

	if err := c.CloseFd(responseDirFd); err != nil {
		db.DFatalf("Run: failed to close response dir fd %v", err)
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

func (c *PerfCoord) clearResponseDir() {
	db.DPrintf(db.WATCH_PERF, "clearResponseDir: Clearing response dir")
	err := c.RmDirEntries(c.responseDir)
	if err != nil {
		db.DFatalf("clearResponseDir: failed to clear response dir entries %v", err)
	}
}

func (c *PerfCoord) getWorkerDelays(startTime time.Time) []time.Duration {
	db.DPrintf(db.WATCH_PERF, "getWorkerDelays: Getting proc times")
	times := c.getWorkerTimes()
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

func (c *PerfCoord) getWorkerTimes() []time.Time {
	times := make([]time.Time, c.nWorkers)
	stats, _, err := c.ReadDir(c.responseDir)
	if err != nil {
		db.DFatalf("getWorkerTimes: failed to read response dir %v", err)
	}

	db.DPrintf(db.WATCH_PERF, "getWorkerTimes: Getting proc times")

	if len(stats) != c.nWorkers {
		db.DFatalf("getWorkerTimes: number of files in %s is not equal to num of workers %d, %v", c.responseDir, c.nWorkers, stats)
	}
	for _, stat := range stats {
		db.DPrintf(db.WATCH_PERF, "getWorkerTimes: Found file %s", stat.Name)
		id, err := strconv.Atoi(stat.Name)
		if err != nil {
			db.DFatalf("getWorkerTimes: file name could not be parsed as proc id: %s, %v", stat.Name, err)
		}

		path := filepath.Join(c.responseDir, stat.Name)
		reader, err := c.OpenReader(path)
		if err != nil {
			db.DFatalf("getWorkerTimes: failed to open file %s, %v", path, err)
		}
		scanner := bufio.NewScanner(reader.Reader)
		exists := scanner.Scan()
		if !exists {
			db.DFatalf("getWorkerTimes: file %s does not contain line", path)
		}

		text := scanner.Text()
		db.DPrintf(db.WATCH_PERF, "getWorkerTimes: File %s has contents %s", stat.Name, text)

		if c.measureMode == JustWatch {
			duration, err := strconv.Atoi(text)
			if err != nil {
				db.DFatalf("getWorkerTimes: failed to parse duration %s, %v", text, err)
			}
			times[id] = time.Unix(0, int64(duration))
		} else {
			time, err := time.Parse(time.RFC3339Nano, text)
			if err != nil {
				db.DFatalf("getWorkerTimes: failed to parse time %s, %v", text, err)
			}
			times[id] = time
		}
	}

	return times
}

func (c *PerfCoord) waitResponses(responseWatcher *fslib.DirWatcher) {
	if c.useOldWatch {
		dirReader := fslib.NewDirReader(c.FsLib, c.responseDir)
		err := dirReader.WaitNEntries(c.nWorkers)
		if err != nil {
			db.DFatalf("Run: failed to wait for all procs to respond %v", err)
		}
	} else {
		err := responseWatcher.WaitNEntries(c.nWorkers)
		if err != nil {
			db.DFatalf("Run: failed to wait for all procs to respond %v", err)
		}
	}
}

func (c *PerfCoord) waitResponseEmpty(responseDirFd int, responseWatcher *fslib.DirWatcher) {
	if c.useOldWatch {
		for {
			stats, _, err := c.ReadDir(c.responseDir)
			if err != nil {
				db.DFatalf("Run: failed to read response dir %v", err)
			}
			if len(stats) == 0 {
				break
			}
			c.DirWatch(responseDirFd)
		}
	} else {
		responseWatcher.WaitEmpty()
	}
}

func coordSignalName(trial int, delete bool) string {
	if delete {
		return fmt.Sprintf("coord_signal_delete_%d", trial)
	}
	return fmt.Sprintf("coord_signal_create_%d", trial)
}
