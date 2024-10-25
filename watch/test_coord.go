package watch

import (
	"fmt"
	"path/filepath"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
	"strconv"
	"sync"
)

type TestCoord struct {
	*sigmaclnt.SigmaClnt
  nWorkers int
	nFiles int
	baseDir string
	watchDir string
	readyDir string
	tempDir string
}

func NewTestCoord(args []string) (*TestCoord, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return &TestCoord{}, fmt.Errorf("NewTestCoord: error %v", err)
	}

	err = sc.Started()
	if err != nil {
		return &TestCoord{}, fmt.Errorf("NewTestCoord: error %v", err)
	}

	nWorkers, err := strconv.Atoi(args[0])
	if err != nil {
		return &TestCoord{}, fmt.Errorf("NewTestCoord: nworkers %s is not an integer", args[0])
	}

	nFiles, err := strconv.Atoi(args[1])
	if err != nil {
		db.DFatalf("Run: nfiles %s is not an integer", args[1])
	}

	baseDir := args[2]
	watchDir := filepath.Join(baseDir, "watch")
	readyDir := filepath.Join(baseDir, "ready")
	tempDir := filepath.Join(baseDir, "temp")

	return &TestCoord{
		sc,
		nWorkers,
		nFiles,
		baseDir,
		watchDir,
		readyDir,
		tempDir,
	}, nil
}

func (c *TestCoord) Run() {
	c.MkDir(c.baseDir, 0777)
	c.MkDir(c.watchDir, 0777)
	c.MkDir(c.tempDir, 0777)
	c.MkDir(c.readyDir, 0777)

	var wg sync.WaitGroup
	sums := make([]uint64, c.nWorkers)

	for ix := 0; ix < c.nWorkers; ix++ {
		wg.Add(1)
		go func (ix int) {
			defer wg.Done()

			p := proc.NewProc("watchtest-worker", []string{strconv.Itoa(ix), strconv.Itoa(c.nFiles), c.watchDir, c.readyDir})
			err := c.Spawn(p)
			if err != nil {
				db.DFatalf("Run: spawning %d failed %v", ix, err)
			}
			err = c.WaitStart(p.GetPid())
			if err != nil {
				db.DFatalf("Run: starting %d failed %v", ix, err)
			}
			status, err := c.WaitExit(p.GetPid())
			if err != nil {
				db.DFatalf("Run: running %d failed %v", ix, err)
			}

			db.DPrintf(db.WATCH_TEST, "Run: got sum for worker %d", ix)
			sums[ix] = uint64(status.Data().(float64))
		}(ix)
	}

	dirWatcher, _, err := fslib.NewDirWatcher(c.FsLib, c.readyDir)
	if err != nil {
		db.DFatalf("Run: failed to create dir watcher for ready dir %v", err)
	}
	db.DPrintf(db.WATCH_TEST, "Run: waiting for %d workers", c.nWorkers)
	err = dirWatcher.WaitNEntries(c.nWorkers)
	if err != nil {
		db.DFatalf("Run: failed to wait for all procs to be ready %v", err)
	}
	err = dirWatcher.Close()
	db.DPrintf(db.WATCH_TEST, "Run: all workers ready")
	if err != nil {
		db.DFatalf("Run: failed to close watcher %v", err)
	}

	db.DPrintf(db.WATCH_TEST, "Run: creating %d files", c.nFiles)
	sum := uint64(0)
	for ix := 0; ix < c.nFiles; ix++ {
		randInt := rand.Int64(1000000)

		tempPath := inputFilepath(c.tempDir, ix)
		path := inputFilepath(c.watchDir, ix)

		fd, err := c.Create(tempPath, 0777, sigmap.OWRITE)
		if err != nil {
			db.DFatalf("Run: failed to create file %d %v", ix, err)
		}
		asciiStr := strconv.FormatUint(randInt, 10)
    bytes := []byte(asciiStr)

		c.Write(fd, bytes)
		c.CloseFd(fd)

		if err = c.Rename(tempPath, path); err != nil {
			db.DFatalf("Run: failed to rename file %d %v", ix, err)
		}

		sum += randInt
	}

	wg.Wait()

	for ix := 0; ix < c.nFiles; ix++ {
		path := inputFilepath(c.watchDir, ix)
		err = c.Remove(path)
		if err != nil {
			db.DFatalf("Run: failed to remove %s, %v", path, err)
		}
	}

	if err := c.Remove(c.watchDir); err != nil {
		db.DFatalf("Run: failed to remove watchdir %v", err)
	}
	if err := c.Remove(c.tempDir); err != nil {
		db.DFatalf("Run: failed to remove tempdir %v", err)
	}
	if err := c.Remove(c.readyDir); err != nil {
		db.DFatalf("Run: failed to remove readydir %v", err)
	}
	if err := c.Remove(c.baseDir); err != nil {
		db.DFatalf("Run: failed to remove basedir %v", err)
	}

	failed := make([]int, 0, c.nWorkers)
	for ix := 0; ix < c.nWorkers; ix++ {
		if sums[ix] != sum {
			db.DPrintf(db.WATCH_TEST, "Run: proc %d did not match %d != %d", ix, sums[ix], sum)
			failed = append(failed, ix)
		}
	}
	if len(failed) > 0 {
		db.DFatalf("Run: some children failed to get correct sum %v", failed)
	}

	status := proc.NewStatusInfo(proc.StatusOK, "", sum)
	c.ClntExit(status)
}

func inputFilepath(watchdir string, ix int) string {
	return filepath.Join(watchdir, fmt.Sprintf("input_%d", ix))
}