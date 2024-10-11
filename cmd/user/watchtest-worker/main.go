package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v id nfiles workdir readydir\n", os.Args[0])
	}

	RunWorker(os.Args[1:])
}

func RunWorker(args []string) {
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

	db.DPrintf(db.WATCH_TEST, "RunWorker: %v\n", args)

	idFilePath := filepath.Join(readyDir, id)
	idFileFd, err := sc.Create(idFilePath, 0777, sigmap.OAPPEND)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to create id file %v", id, err)
	}

	workDirFd, err := sc.Open(workDir, 0777)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to open workDir %s: %v", id, workDir, err)
	}

	sum := uint64(0)
	seen := make(map[string]bool)
	dirWatcher, initFiles, err := fslib.NewDirWatcher(sc.FsLib, workDir)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to create dir watcher for %s: %v", id, workDir, err)
	}

	for _, file := range initFiles {
		if !seen[file] {
			sum += readFile(sc, id, filepath.Join(workDir, file))
			seen[file] = true
		} else {
			db.DPrintf(db.WATCH_TEST, "RunWorker %s: found duplicate %s", id, file)
		}
	}
	for {
		changed, err := dirWatcher.WatchEntriesChanged()
		if err != nil {
			db.DFatalf("RunWorker %s: failed to watch for entries changed %v", id, err)
		}
		addedFiles := make([]string, 0)
		for file, created := range changed {
			if created {
				addedFiles = append(addedFiles, file)
			}
		}
		db.DPrintf(db.WATCH_TEST, "RunWorker %s: added files: %v", id, addedFiles)

		for _, file := range addedFiles {
			if !seen[file] {
				sum += readFile(sc, id, filepath.Join(workDir, file))
				seen[file] = true
			} else {
				db.DPrintf(db.WATCH_TEST, "RunWorker %s: found duplicate %s", id, file)
			}
		}

		if len(seen) >= nfiles {
			break
		}
	}

	err = sc.CloseFd(workDirFd)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to close fd for workDir %v", id, err)
	}
	
	err = sc.CloseFd(idFileFd)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to close fd for id file %v", id, err)
	}

	err = sc.Remove(idFilePath)
	if err != nil {
		db.DFatalf("RunWorker %s: failed to delete id file %v", id, err)
	}
	
	status := proc.NewStatusInfo(proc.StatusOK, "", sum)
	sc.ClntExit(status)
}

func readFile(sc *sigmaclnt.SigmaClnt, id string, file string) uint64 {
	reader, err := sc.OpenReader(file)
	if err != nil {
		db.DFatalf("readFile id %s: failed to open file: err %v", id, err)
	}
	scanner := bufio.NewScanner(reader.Reader)
	exists := scanner.Scan()
	if !exists {
		db.DFatalf("readFile id %s: file %s does not contain line", id, file)
	}

	line := scanner.Text()
	num, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		db.DFatalf("readFile id %s: failed to parse %v as u64: err %v", id, line, err)
	}

	if err = reader.Close(); err != nil {
		db.DFatalf("readFile id %s: failed to close reader for %s: err %v", id, file, err)
	}

	return num
}

