package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("Usage: %v nworkers nfiles basedir\n", os.Args[0])
	}

	RunCoord(os.Args[1:])
}

func RunCoord(args []string) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt: error %v\n", err)
	}

	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}

	nWorkers, err := strconv.Atoi(args[0])
	if err != nil {
		db.DFatalf("RunCoord: nworkers %s is not an integer", args[0])
	}

	nFiles, err := strconv.Atoi(args[1])
	if err != nil {
		db.DFatalf("RunCoord: nfiles %s is not an integer", args[1])
	}

	baseDir := args[2]
	watchDir := filepath.Join(baseDir, "watch")
	readyDir := filepath.Join(baseDir, "ready")
	tempDir := filepath.Join(baseDir, "temp")

	db.DPrintf(db.WATCH_TEST, "RunCoord: %v\n", args)

	sc.MkDir(baseDir, 0777)
	sc.MkDir(watchDir, 0777)
	sc.MkDir(tempDir, 0777)
	sc.MkDir(readyDir, 0777)

	var wg sync.WaitGroup
	sums := make([]uint64, nWorkers)

	for ix := 0; ix < nWorkers; ix++ {
		wg.Add(1)
		go func (ix int) {
			defer wg.Done()

			p := proc.NewProc("watchtest-worker", []string{strconv.Itoa(ix), strconv.Itoa(nFiles), watchDir, readyDir})
			err = sc.Spawn(p)
			if err != nil {
				db.DFatalf("RunCoord: spawning %d failed %v", ix, err)
			}
			err = sc.WaitStart(p.GetPid())
			if err != nil {
				db.DFatalf("RunCoord: starting %d failed %v", ix, err)
			}
			status, err := sc.WaitExit(p.GetPid())
			if err != nil {
				db.DFatalf("RunCoord: running %d failed %v", ix, err)
			}

			sums[ix] = uint64(status.Data().(float64))
		}(ix)
	}

	dirWatcher, err := fslib.NewDirWatcher(sc.FsLib, readyDir)
	if err != nil {
		db.DFatalf("RunCoord: failed to create dir watcher for ready dir %v", err)
	}
	err = dirWatcher.WaitNEntries(nWorkers)
	if err != nil {
		db.DFatalf("RunCoord: failed to wait for all procs to be ready %v", err)
	}

	sum := uint64(0)
	for ix := 0; ix < nFiles; ix++ {
		randInt := rand.Int64(1000000)

		tempPath := inputFilepath(tempDir, ix)
		path := inputFilepath(watchDir, ix)

		fd, err := sc.Create(tempPath, 0777, sigmap.OWRITE)
		if err != nil {
			db.DFatalf("RunCoord: failed to create file %d %v", ix, err)
		}
		asciiStr := strconv.FormatUint(randInt, 10)
    bytes := []byte(asciiStr)

		sc.Write(fd, bytes)
		sc.CloseFd(fd)

		if err = sc.Rename(tempPath, path); err != nil {
			db.DFatalf("RunCoord: failed to rename file %d %v", ix, err)
		}

		sum += randInt
	}

	wg.Wait()

	for ix := 0; ix < nFiles; ix++ {
		path := inputFilepath(watchDir, ix)
		err = sc.Remove(path)
		if err != nil {
			db.DFatalf("RunCoord: failed to remove %s, %v", path, err)
		}
	}

	if sc.Remove(watchDir) != nil {
		db.DFatalf("RunCoord: failed to remove watchdir %v", err)
	}
	if sc.Remove(tempDir) != nil {
		db.DFatalf("RunCoord: failed to remove tempdir %v", err)
	}
	if sc.Remove(readyDir) != nil {
		db.DFatalf("RunCoord: failed to remove readydir %v", err)
	}
	if sc.Remove(tempDir) != nil {
		db.DFatalf("RunCoord: failed to remove tempdir %v", err)
	}
	if sc.Remove(baseDir) != nil {
		db.DFatalf("RunCoord: failed to remove basedir %v", err)
	}

	failed := make([]int, 0, nWorkers)
	for ix := 0; ix < nWorkers; ix++ {
		if sums[ix] != sum {
			failed = append(failed, ix)
		}
	}
	if len(failed) > 0 {
		db.DFatalf("RunCoord: some children failed to get correct sum %v", failed)
	}

	status := proc.NewStatusInfo(proc.StatusOK, "", sum)
	sc.ClntExit(status)
}

func inputFilepath(watchdir string, ix int) string {
	return filepath.Join(watchdir, fmt.Sprintf("input_%d", ix))
}
