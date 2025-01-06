package test

import (
	"fmt"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib/dirreader"
	"sigmaos/sigmap"
	"strconv"
	"time"
)

type PerfWorker struct {
	*sigmaclnt.SigmaClnt
	id string;
	nTrials int;
	nFilesPerTrial int;
	watchDir string;
	watchDirReader *dirreader.DirReader;
	responseDir string;
	tempDir string;
	signalDir string;
	signalDirReader *dirreader.DirReader;
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

	nFilesPerTrial, err := strconv.Atoi(args[2])
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: nFilesPerTrial %s is not an integer", id, args[2])
	}

	watchDir := args[3]
	responseDir := args[4]
	tempDir := args[5]
	signalDir := args[6]

	watchDirReader, err := dirreader.NewDirReader(sc.FsLib, watchDir)
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: failed to construct watcher for %s, %v", id, watchDir, err)
	}
	signalDirReader, err := dirreader.NewDirReader(sc.FsLib, signalDir)
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: failed to construct watcher for %s, %v", id, signalDir, err)
	}

	measureMode, err := strconv.Atoi(args[7])
	if err != nil || measureMode < 0 || measureMode > 1 {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker: measure mode %s is invalid", args[6])
	}

	nStartFiles, err := strconv.Atoi(args[8])
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker: nStartFiles %s is not an integer", args[7])
	}

	return &PerfWorker {
		sc,
		id,
		nTrials,
		nFilesPerTrial,
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
		w.handleTrial(trial, false)
		w.handleTrial(trial, true)
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

func (w *PerfWorker) handleTrial(trial int, deleted bool) {
	opType := "creation"
	if deleted {
		opType = "deletion"
	}

	db.DPrintf(db.WATCH_PERF, "handleTrial %s: Trial %d: waiting for file %s", w.id, trial, opType)
	w.waitForCoordSignal(trial, deleted)

	times := make([]time.Time, w.nFilesPerTrial)
	if w.measureMode == IncludeFileOp {
		ch := make(chan bool)
		go func() {
			w.waitForWatchFile(trial, deleted, times)
			ch <- true
		}()

		// some delay that is much longer than the time it would take for the watch to start
		time.Sleep(100 * time.Millisecond) 

		w.sendSignal(trial, deleted)
		<- ch
		w.respondWith(formatTimes(times), trial, deleted)
	} else {
		preWatchTime := time.Now()
		w.waitForWatchFile(trial, deleted, times)
		w.respondWith(formatDurations(times, preWatchTime), trial, deleted)
	}
}

// wait for a signal from the coord that we should now start watching for the next file
func (w *PerfWorker) waitForCoordSignal(trial int, deleted bool) {
	w.waitForFile(w.signalDirReader, coordSignalName(trial, deleted), false)
}

func (w *PerfWorker) waitForWatchFile(trial int, deleted bool, times []time.Time) {
	opType := "creation"
	if deleted {
		opType = "deletion"
	}

	for {
		changes, err := w.watchDirReader.WatchEntriesChanged()
		if err != nil {
			db.DFatalf("handleTrial %s: failed to watch entries changed, %v", w.id, err)
		}
		db.DPrintf(db.WATCH_PERF, "handleTrial %s: trial %d, received changes %v", w.id, trial, changes)

		anyMissing := false
		for ix := 0; ix < w.nFilesPerTrial; ix++ {
			if !times[ix].IsZero() {
				continue
			}

			created, ok := changes[trialName(trial, ix)]
			if !ok {
				anyMissing = true
				continue
			}

			if created != !deleted {
				db.DFatalf("handleTrial %s: trial %d, expected file to be %s, but received opposite", w.id, trial, opType)
			}

			db.DPrintf(db.WATCH_PERF, "handleTrial %s: trial %d, file %d %s", w.id, trial, ix, opType)
			times[ix] = time.Now()
		}

		if !anyMissing {
			break
		}
	}
}

func (w *PerfWorker) waitForFile(watcher *dirreader.DirReader, filename string, deleted bool) {
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

func formatTimes(times []time.Time) string {
	str := ""
	for _, t := range times {
		str += t.Format(time.RFC3339Nano) + "\n"
	}
	return str
}

func formatDurations(times []time.Time, startTime time.Time) string {
	durations := make([]time.Duration, len(times))
	for ix, t := range times {
		durations[ix] = t.Sub(startTime)
	}
	str := ""
	for _, d := range durations {
		str += strconv.FormatInt(d.Nanoseconds(), 10) + "\n"
	}

	return str
}