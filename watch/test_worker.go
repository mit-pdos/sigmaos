package watch

import (
	"bufio"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
	"strconv"
)

type TestWorker struct {
	*sigmaclnt.SigmaClnt
	id string;
	nfiles int;
	workDir string;
	readyDir string;
}

func NewTestWorker(args []string) (*TestWorker, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt: error %v\n", err)
	}

	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}

	id := args[0]

	nfiles, err := strconv.Atoi(args[1])
	if err != nil {
		db.DFatalf("RunWorker %s: nfiles %s is not an integer", id, args[1])
	}

	workDir := args[2]
	readyDir := args[3]

	return &TestWorker {
		sc,
		id,
		nfiles,
		workDir,
		readyDir,
	}, nil
}

func (w *TestWorker) Run() {
	idFilePath := filepath.Join(w.readyDir, w.id)
	idFileFd, err := w.Create(idFilePath, 0777, sigmap.OAPPEND)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to create id file %v", w.id, err)
	}

	workDirFd, err := w.Open(w.workDir, 0777)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to open workDir %s: %v", w.id, w.workDir, err)
	}

	sum := uint64(0)
	seen := make(map[string]bool)
	dirWatcher, initFiles, err := fslib.NewDirWatcher(w.FsLib, w.workDir)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to create dir watcher for %s: %v", w.id, w.workDir, err)
	}

	for _, file := range initFiles {
		if !seen[file] {
			sum += w.readFile(filepath.Join(w.workDir, file))
			seen[file] = true
		} else {
			db.DPrintf(db.WATCH_TEST, "RunWorker %s: found duplicate %s", w.id, file)
		}
	}
	for {
		changed, err := dirWatcher.WatchEntriesChanged()
		if err != nil {
			db.DFatalf("RunWorker %s: failed to watch for entries changed %v", w.id, err)
		}
		addedFiles := make([]string, 0)
		for file, created := range changed {
			if created {
				addedFiles = append(addedFiles, file)
			}
		}
		db.DPrintf(db.WATCH_TEST, "RunWorker %s: added files: %v", w.id, addedFiles)

		for _, file := range addedFiles {
			if !seen[file] {
				sum += w.readFile(filepath.Join(w.workDir, file))
				seen[file] = true
			} else {
				db.DPrintf(db.WATCH_TEST, "RunWorker %s: found duplicate %s", w.id, file)
			}
		}

		if len(seen) >= w.nfiles {
			break
		}
	}

	err = dirWatcher.Close()
	if err != nil {
		db.DFatalf("RunWorker %s: failed to close dir watcher", err)
	}

	err = w.CloseFd(workDirFd)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to close fd for workDir %v", w.id, err)
	}
	
	err = w.CloseFd(idFileFd)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to close fd for id file %v", w.id, err)
	}

	err = w.Remove(idFilePath)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to delete id file %v", w.id, err)
	}
	
	status := proc.NewStatusInfo(proc.StatusOK, "", sum)
	w.ClntExit(status)
}

func (w *TestWorker) readFile(file string) uint64 {
	reader, err := w.OpenReader(file)
	if err != nil {
		db.DFatalf("readFile id %s: failed to open file: err %v", w.id, err)
	}
	scanner := bufio.NewScanner(reader.Reader)
	exists := scanner.Scan()
	if !exists {
		db.DFatalf("readFile id %s: file %s does not contain line", w.id, file)
	}

	line := scanner.Text()
	num, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		db.DFatalf("readFile id %s: failed to parse %v as u64: err %v", w.id, line, err)
	}

	if err = reader.Close(); err != nil {
		db.DFatalf("readFile id %s: failed to close reader for %s: err %v", w.id, file, err)
	}

	return num
}

