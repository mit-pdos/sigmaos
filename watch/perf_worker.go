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
	watchDirWatcher *fslib.DirWatcher;
	responseDir string;
	tempDir string;
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

	watchDirWatcher, _, err := fslib.NewDirWatcher(sc.FsLib, watchDir)
	if err != nil {
		return &PerfWorker{}, fmt.Errorf("NewPerfWorker %s: failed to construct watcher for %s, %v", id, watchDir, err)
	}

	return &PerfWorker {
		sc,
		id,
		nTrials,
		watchDir,
		watchDirWatcher,
		responseDir,
		tempDir,
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
	for trial := 0; trial < w.nTrials; trial++ {
		filename := fmt.Sprintf("trial_%d", trial)
		db.DPrintf(db.WATCH_PERF, "Run %s: Trial %d: waiting for file creation", w.id, trial)
		w.waitForFile(w.watchDir, filename, false)
		createdFileTime := time.Now()
		w.respondWithTime(createdFileTime)

		db.DPrintf(db.WATCH_PERF, "Run %s: Trial %d: waiting for file deletion", w.id, trial)
		w.waitForFile(w.watchDir, filename, true)
		deletedFileTime := time.Now()
		w.respondWithTime(deletedFileTime)
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

func (w *PerfWorker) waitForFile(watchDir string, filename string, deleted bool) {
	db.DPrintf(db.WATCH_PERF, "waitForFile: waiting for %s/%s (deleted = %t)", watchDir, filename, deleted)

	var err error
	if deleted {
		err = w.watchDirWatcher.WaitRemove(filename)
	} else {
		err = w.watchDirWatcher.WaitCreate(filename)
	}

	if err != nil {
		db.DFatalf("waitForFile: failed to wait for %s/%s: err %v", watchDir, filename, err)
	}
}

func (w *PerfWorker) respondWithTime(responseTime time.Time) {
	db.DPrintf(db.WATCH_PERF, "respondWithTime: id %s with time %v", w.id, responseTime)
	tempPath := filepath.Join(w.tempDir, w.id)
	realPath := filepath.Join(w.responseDir, w.id)
	fd, err := w.Create(tempPath, 0777, sigmap.OAPPEND)
	if err != nil {
		db.DFatalf("respondWithTime: failed to create file %s, %v", tempPath, err)
	}
	bytes := []byte(responseTime.Format(time.RFC3339Nano))

	w.Write(fd, bytes)
	w.CloseFd(fd)

	err = w.Rename(tempPath, realPath)
	if err != nil {
		db.DFatalf("respondWithTime: failed to rename %s to %s, %v", tempPath, realPath, err)
	}
}