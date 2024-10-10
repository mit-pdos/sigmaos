package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	db "sigmaos/debug"
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
		db.DFatalf("RunWorker %s: failed to open workDir %s", id, workDir)
	}

	sum := uint64(0)
	nfilesSeen := 0
	for {
		if err != nil {
			db.DFatalf("RunWorker %s: failed to wait for %s", id, workDir)
		}

		stats, err := sc.GetDir(workDir)
		if err != nil {
			db.DFatalf("RunWorker %s: failed to get dir: err %v", id, err)
		}

		sum = 0
		nfilesSeen = 0
		for _, stat := range stats {
			if strings.Contains(stat.Name, "input_") {
				file := filepath.Join(workDir, stat.Name)
				reader, err := sc.OpenReader(file)
				if err != nil {
					db.DFatalf("RunWorker %s: failed to open file: err %v", id, err)
				}
				scanner := bufio.NewScanner(reader.Reader)
				exists := scanner.Scan()
				if !exists {
					db.DFatalf("RunWorker %s: file %s does not contain line", id, file)
				}

				line := scanner.Text()
				num, err := strconv.ParseUint(line, 10, 64)
				if err != nil {
					db.DFatalf("RunWorker %s: failed to parse %v as u64: err %v", id, line, err)
				}

				if err = reader.Close(); err != nil {
					db.DFatalf("RunWorker %s: failed to close reader for %s: err %v", id, file, err)
				}

				sum += num
				nfilesSeen++
			}
		}

		db.DPrintf(db.WATCH_TEST, "RunWorker %s: found %d files", id, nfilesSeen)

		if nfilesSeen >= nfiles {
			break;
		}
		db.DPrintf(db.WATCH_TEST, "RunWorker %s: watching %s", id, workDir)
		sc.DirWatch(workDirFd)
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

