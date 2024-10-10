package watch

import (
	"fmt"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
	"strconv"
	"time"
)

type Worker struct {
	*sigmaclnt.SigmaClnt
	id string;
	nTrials int;
	watchDir string;
	responseDir string;
	tempDir string;
}

func NewWorker(args []string) (*Worker, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return &Worker{}, fmt.Errorf("NewWorker: error %v", err)
	}

	err = sc.Started()
	if err != nil {
		return &Worker{}, fmt.Errorf("NewWorker: error %v", err)
	}

	id := args[0]

	nTrials, err := strconv.Atoi(args[1])
	if err != nil {
		return &Worker{}, fmt.Errorf("NewWorker %s: ntrials %s is not an integer", id, args[1])
	}

	watchDir := args[2]
	responseDir := args[3]
	tempDir := args[4]

	return &Worker {
		sc,
		id,
		nTrials,
		watchDir,
		responseDir,
		tempDir,
	}, nil
}

func (w *Worker) Run() {
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
		w.waitForFile(watchDirFd, w.watchDir, filename, false)
		createdFileTime := time.Now()
		w.respondWithTime(createdFileTime)

		db.DPrintf(db.WATCH_PERF, "Run %s: Trial %d: waiting for file deletion", w.id, trial)
		w.waitForFile(watchDirFd, w.watchDir, filename, true)
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

func (w *Worker) waitForFile(watchFd int, watchDir string, filename string, deleted bool) {
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
}

func (w *Worker) respondWithTime(responseTime time.Time) {
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