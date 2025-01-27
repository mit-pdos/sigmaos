package test

import (
	"fmt"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib/dircache"
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
	watchDirCache *dircache.DirCache[time.Time];
	responseDir string;
	tempDir string;
	signalDir string;
	signalDirCache *dircache.DirCache[struct{}];
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

	watchDirCache := dircache.NewDirCache(sc.FsLib, watchDir, func(_ string) (time.Time, error) {
		return time.Now(), nil
	}, nil, db.WATCH_PERF, db.WATCH_PERF)
	watchDirCache.Init()
	signalDirCache := dircache.NewDirCache(sc.FsLib, signalDir, func(_ string) (struct{}, error) {
		return struct{}{}, nil
	}, nil, db.WATCH_PERF, db.WATCH_PERF)
	signalDirCache.Init()

	nStartFiles, err := strconv.Atoi(args[7])
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker: nStartFiles %s is not an integer", args[7])
	}

	return &PerfWorker {
		sc,
		id,
		nTrials,
		nFilesPerTrial,
		watchDir,
		watchDirCache,
		responseDir,
		tempDir,
		signalDir,
		signalDirCache,
		nStartFiles,
	}, nil
}

func (w *PerfWorker) Run() {
	_, err := w.watchDirCache.WaitEntriesN(w.nStartFiles, false)
	if err != nil {
		db.DFatalf("Run %s: failed to wait for %d start files in %s, %v", w.id, w.nStartFiles, w.watchDir, err)
	}

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

	w.watchDirCache.StopWatching()
	w.signalDirCache.StopWatching()
	
	status := proc.NewStatusInfo(proc.StatusOK, "", nil)
	w.ClntExit(status)
}

func (w *PerfWorker) handleTrial(trial int, deleted bool) {
	opType := "creation"
	if deleted {
		opType = "deletion"
	}

	db.DPrintf(db.WATCH_PERF, "handleTrial %s: Trial %d: waiting for file %s", w.id, trial, opType)
	times := make([]time.Time, w.nFilesPerTrial)
	w.waitForWatchFile(trial, deleted, times)
	w.respondWith(formatTimes(times), trial, deleted)
}

func (w *PerfWorker) waitForWatchFile(trial int, deleted bool, times []time.Time) {
	opType := "creation"
	if deleted {
		opType = "deletion"
	}

	allFiles := make([]string, w.nFilesPerTrial)
	for ix := 0; ix < w.nFilesPerTrial; ix++ {
		allFiles[ix] = trialName(trial, ix)
	}

	db.DPrintf(db.WATCH_PERF, "waitForWatchFile %s: waiting for %s of all files in trial %d: %v", w.id, opType, trial, allFiles)
	var err error
	if !deleted {
		err = w.watchDirCache.WaitAllEntriesCreated(allFiles)
	} else {
		err = w.watchDirCache.WaitAllEntriesRemoved(allFiles)
	}
	if err != nil {
		db.DFatalf("handleTrial %s: failed to wait for %s of all files in trial %d, %v", w.id, opType, trial, err)
	}

	if !deleted {
		for ix := 0; ix < w.nFilesPerTrial; ix++ {
			times[ix], err = w.watchDirCache.GetEntry(allFiles[ix])
			if err != nil {
				db.DFatalf("handleTrial %s: failed to get time for file %s, %v", w.id, allFiles[ix], err)
			}
		}
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

func formatTimes(times []time.Time) string {
	str := ""
	for _, t := range times {
		str += t.Format(time.RFC3339Nano) + "\n"
	}
	return str
}
