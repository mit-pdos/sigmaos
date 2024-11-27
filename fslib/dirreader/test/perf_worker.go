package test

import (
	"fmt"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/fslib/dirreader"
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
	watchDirReader dirreader.DirReader;
	responseDir string;
	tempDir string;
	signalDir string;
	signalDirReader dirreader.DirReader;
	measureMode MeasureMode;
	nStartFiles int;
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

	watchDirReader, err := dirreader.NewDirReader(sc.FsLib, watchDir)
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: failed to construct watcher for %s, %v", id, watchDir, err)
	}
	signalDirReader, err := dirreader.NewDirReader(sc.FsLib, signalDir)
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: failed to construct watcher for %s, %v", id, signalDir, err)
	}

	measureMode, err := strconv.Atoi(args[6])
	if err != nil || measureMode < 0 || measureMode > 1 {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker: measure mode %s is invalid", args[6])
	}

	nStartFiles, err := strconv.Atoi(args[7])
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker: nStartFiles %s is not an integer", args[7])
	}

	return &PerfWorker {
		sc,
		id,
		nTrials,
		watchDir,
		watchDirReader,
		responseDir,
		tempDir,
		signalDir,
		signalDirReader,
		MeasureMode(measureMode),
		nStartFiles,
	}, nil
}

func (w *PerfWorker) Run() {
	w.watchDirReader.WaitNEntries(w.nStartFiles)

	idFilePath := filepath.Join(w.responseDir, w.id)
	idFileFd, err := w.Create(idFilePath, 0777, sigmap.OAPPEND)
	if err != nil {
		db.DFatalf("Run %s: failed to create id file %v", w.id, err)
	}

	for trial := 0; trial < w.nTrials; trial++ {
		filename := fmt.Sprintf("trial_%d", trial)
		w.handleTrial(trial, filename, false)
		w.handleTrial(trial, filename, true)
	}

	err = w.CloseFd(idFileFd)
	if err != nil {
		db.DFatalf("Run %s: failed to close fd for id file %v", w.id, err)
	}

	err = w.watchDirReader.Close()
	if err != nil {
		db.DFatalf("Run %s: failed to close watcher for %s, %v", w.id, w.watchDir, err)
	}

	err = w.signalDirReader.Close()
	if err != nil {
		db.DFatalf("Run %s: failed to close watcher for %s, %v", w.id, w.signalDir, err)
	}
	
	status := proc.NewStatusInfo(proc.StatusOK, "", nil)
	w.ClntExit(status)
}

func (w *PerfWorker) handleTrial(trial int, filename string, deleted bool) {
	opType := "creation"
	if deleted {
		opType = "deletion"
	}

	db.DPrintf(db.WATCH_PERF, "handleTrial %s: Trial %d: waiting for file %s", w.id, trial, opType)
	w.waitForCoordSignal(trial, deleted)

	if w.measureMode == IncludeFileOp {
		ch := make(chan bool)
		go func() {
			w.waitForWatchFile(filename, deleted)
			ch <- true
		}()

		// some delay that is much longer than the time it would take for the watch to start
		time.Sleep(100 * time.Millisecond) 

		w.sendSignal(trial, deleted)
		<- ch
		createdFileTime := time.Now()
		w.respondWith(formatTime(createdFileTime), trial, deleted)
	} else {
		preWatchTime := time.Now()
		w.waitForWatchFile(filename, deleted)
		createdFileTime := time.Now()
		w.respondWith(formatDuration(createdFileTime.Sub(preWatchTime)), trial, deleted)
	}
}

// wait for a signal from the coord that we should now start watching for the next file
func (w *PerfWorker) waitForCoordSignal(trial int, deleted bool) {
	w.waitForFile( w.signalDirReader, coordSignalName(trial, deleted), false)
}

func (w *PerfWorker) waitForWatchFile(filename string, deleted bool) {
	w.waitForFile( w.watchDirReader, filename, deleted)
}

func (w *PerfWorker) waitForFile(watcher dirreader.DirReader, filename string, deleted bool) {
	db.DPrintf(db.WATCH_PERF, "waitForFile: waiting for %s/%s (deleted = %t)", w.watchDir, filename, deleted)

	var err error
	if deleted {
		err = watcher.WaitRemove(filename)
	} else {
		err = watcher.WaitCreate(filename)
	}

	if err != nil {
		db.DFatalf("waitForFile: failed to wait for %s/%s: err %v", watcher.GetPath(), filename, err)
	}
}

func (w *PerfWorker) respondWith(resp string, trialNum int, deleted bool) {
	db.DPrintf(db.WATCH_PERF, "respondWithTime: id %s with resp %s", w.id, resp)
	fileName := responseName(trialNum, w.id, deleted)
	tempPath := filepath.Join(w.tempDir, fileName)
	realPath := filepath.Join(w.responseDir, fileName)
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

	db.DPrintf(db.WATCH_PERF, "respondWithTime: id %s with resp %s to file %s", w.id, resp, realPath)
}

func (w *PerfWorker) sendSignal(trialNum int, deleted bool) {
	signalPath := filepath.Join(w.signalDir, workerSignalName(trialNum, deleted, w.id))
	fd, err := w.Create(signalPath, 0777, sigmap.OAPPEND)
	if err != nil {
		db.DFatalf("sendSignal: failed to create signal file %s, %v", signalPath, err)
	}
	w.CloseFd(fd)
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func formatDuration(d time.Duration) string {
	return strconv.FormatInt(d.Nanoseconds(), 10)
}