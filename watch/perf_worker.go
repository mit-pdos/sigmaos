package watch

import (
	"fmt"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
	"strconv"
	"time"
)

type PerfWorker struct {
	*sigmaclnt.SigmaClnt
	id string;
	nTrials int;
	watchDir string;
	watchDirWatcher *fslib.DirReaderV2;
	responseDir string;
	tempDir string;
	signalDir string;
	signalDirWatcher *fslib.DirReaderV2;
	useOldWatch bool;
	measureMode MeasureMode
}

func NewPerfWorker(args []string) (*PerfWorker, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker: error %v", err)
	}

	err = sc.Started()
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker: error %v", err)
	}

	id := args[0]

	nTrials, err := strconv.Atoi(args[1])
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: ntrials %s is not an integer", id, args[1])
	}

	watchDir := args[2]
	responseDir := args[3]
	tempDir := args[4]
	signalDir := args[5]
	oldOrNew := args[6]

	var useOldWatch bool
	var watchDirWatcher *fslib.DirReaderV2
	var signalDirWatcher *fslib.DirReaderV2

	if oldOrNew == "old" {
		useOldWatch = true
	} else if oldOrNew == "new" {
		useOldWatch = false
		watchDirWatcher, _, err = fslib.NewDirReaderV2(sc.FsLib, watchDir)
		if err != nil {
			return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: failed to construct watcher for %s, %v", id, watchDir, err)
		}
		signalDirWatcher, _, err = fslib.NewDirReaderV2(sc.FsLib, signalDir)
		if err != nil {
			return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: failed to construct watcher for %s, %v", id, signalDir, err)
		}
	} else {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: oldornew %s is not either 'old' or 'new'", id, oldOrNew)
	}

	measureMode, err := strconv.Atoi(args[7])
	if err != nil || measureMode < 0 || measureMode > 1 {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker: measure mode %s is invalid", args[6])
	}

	return &PerfWorker {
		sc,
		id,
		nTrials,
		watchDir,
		watchDirWatcher,
		responseDir,
		tempDir,
		signalDir,
		signalDirWatcher,
		useOldWatch,
		MeasureMode(measureMode),
	}, nil
}

func (w *PerfWorker) Run() {
	idFilePath := filepath.Join(w.responseDir, w.id)
	idFileFd, err := w.Create(idFilePath, 0777, sigmap.OAPPEND)
	if err != nil {
		db.DFatalf("Run %s: failed to create id file %v", w.id, err)
	}

	watchDirFd, err := w.Open(w.watchDir, 0777)
	if err != nil {
		db.DFatalf("Run %s: failed to open watchdir %s", w.id, w.watchDir)
	}
	signalDirFd, err := w.Open(w.signalDir, 0777)
	if err != nil {
		db.DFatalf("Run %s: failed to open signaldir %s", w.id, w.signalDir)
	}
	for trial := 0; trial < w.nTrials; trial++ {
		filename := fmt.Sprintf("trial_%d", trial)

		db.DPrintf(db.WATCH_PERF, "Run %s: Trial %d: waiting for file creation", w.id, trial)
		w.waitForCoordSignal(signalDirFd, trial, false)
		preWatchTime := time.Now()
		w.waitForWatchFile(watchDirFd, filename, false)
		createdFileTime := time.Now()

		if w.measureMode == JustWatch {
			w.respondWith(formatDuration(createdFileTime.Sub(preWatchTime)))
		} else if w.measureMode == IncludeFileOp {
			w.respondWith(formatTime(createdFileTime))
		}

		db.DPrintf(db.WATCH_PERF, "Run %s: Trial %d: waiting for file deletion", w.id, trial)
		w.waitForCoordSignal(signalDirFd, trial, true)
		preWatchTime = time.Now()
		w.waitForWatchFile(watchDirFd, filename, true)
		deletedFileTime := time.Now()

		if w.measureMode == JustWatch {
			w.respondWith(formatDuration(deletedFileTime.Sub(preWatchTime)))
		} else if w.measureMode == IncludeFileOp {
			w.respondWith(formatTime(deletedFileTime))
		}
	}

	err = w.CloseFd(watchDirFd)
	if err != nil {
		db.DFatalf("Run %s: failed to close fd for watchDir %v", w.id, err)
	}
	
	err = w.CloseFd(idFileFd)
	if err != nil {
		db.DFatalf("Run %s: failed to close fd for id file %v", w.id, err)
	}
	
	status := proc.NewStatusInfo(proc.StatusOK, "", nil)
	w.ClntExit(status)
}

// wait for a signal from the coord that we should now start watching for the next file
func (w *PerfWorker) waitForCoordSignal(signalDirFd int, trial int, deleted bool) {
	w.waitForFile(signalDirFd, w.signalDir, w.signalDirWatcher, coordSignalName(trial, deleted), false)
}

func (w *PerfWorker) waitForWatchFile(watchDirFd int, filename string, deleted bool) {
	w.waitForFile(watchDirFd, w.watchDir, w.watchDirWatcher, filename, deleted)
}

func (w *PerfWorker) waitForFile(watchFd int, watchDir string, watcher *fslib.DirReaderV2, filename string, deleted bool) {
	db.DPrintf(db.WATCH_PERF, "waitForFile: waiting for %s/%s (deleted = %t)", w.watchDir, filename, deleted)

	if w.useOldWatch {
		found := false
		for !found {
			found = deleted
			w.DirWatch(watchFd)
			stats, err := w.GetDir(watchDir)
			if err != nil {
				db.DFatalf("waitForFile %s: failed to get dir: err %v", w.id, err)
			}

			for _, stat := range stats {
				if stat.Name == filename {
					found = !deleted
				}
			}
		}
	} else {
		var err error
		if deleted {
			err = watcher.WaitRemove(filename)
		} else {
			err = watcher.WaitCreate(filename)
		}

		if err != nil {
			db.DFatalf("waitForFile: failed to wait for %s/%s: err %v", watchDir, filename, err)
		}
	}
}

func (w *PerfWorker) respondWith(resp string) {
	db.DPrintf(db.WATCH_PERF, "respondWithTime: id %s with resp %s", w.id, resp)
	tempPath := filepath.Join(w.tempDir, w.id)
	realPath := filepath.Join(w.responseDir, w.id)
	fd, err := w.Create(tempPath, 0777, sigmap.OAPPEND)
	if err != nil {
		db.DFatalf("respondWithTime: failed to create file %s, %v", tempPath, err)
	}
	bytes := []byte(resp)

	w.Write(fd, bytes)
	w.CloseFd(fd)

	err = w.Rename(tempPath, realPath)
	if err != nil {
		db.DFatalf("respondWithTime: failed to rename %s to %s, %v", tempPath, realPath, err)
	}
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func formatDuration(d time.Duration) string {
	return strconv.FormatInt(d.Nanoseconds(), 10)
}