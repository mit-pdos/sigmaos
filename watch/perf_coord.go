package watch

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmap"
	"strconv"
	"time"

	db "sigmaos/debug"
)

type Result struct {
	CreationTimeNs [][]time.Duration
	DeletionTimeNs [][]time.Duration
}

type Coord struct {
	*sigmaclnt.SigmaClnt
  nWorkers int
	nStartFiles int
	nTrials int
	baseDir string
	watchDir string
	responseDir string
	tempDir string
}

func NewCoord(args []string) (*Coord, error) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return &Coord{}, fmt.Errorf("NewCoord: error %v", err)
	}

	err = sc.Started()
	if err != nil {
		return &Coord{}, fmt.Errorf("NewCoord: error %v", err)
	}

	nWorkers, err := strconv.Atoi(args[0])
	if err != nil {
		return &Coord{}, fmt.Errorf("NewCoord: nworkers %s is not an integer", args[0])
	}

	nStartFiles, err := strconv.Atoi(args[1])
	if err != nil {
		return &Coord{}, fmt.Errorf("NewCoord: nstartfiles %s is not an integer", args[1])
	}

	nTrials, err := strconv.Atoi(args[2])
	if err != nil {
		return &Coord{}, fmt.Errorf("NewCoord: nTrials %s is not an integer", args[1])
	}

	baseDir := args[3]
	watchDir := filepath.Join(baseDir, "watch")
	responseDir := filepath.Join(baseDir, "response")
	tempDir := filepath.Join(baseDir, "temp")

	return &Coord{
		sc,
		nWorkers,
		nStartFiles,
		nTrials,
		baseDir,
		watchDir,
		responseDir,
		tempDir,
	}, nil
}

func (c *Coord) Run() {
	c.MkDir(c.baseDir, 0777)
	c.MkDir(c.watchDir, 0777)
	c.MkDir(c.responseDir, 0777)
	c.MkDir(c.tempDir, 0777)

	for ix := 0; ix < c.nStartFiles; ix++ {
		path := filepath.Join(c.watchDir, strconv.Itoa(ix))
		fd, err := c.Create(path, 0777, sigmap.OAPPEND)
		if err != nil {
			db.DFatalf("Run: failed to create starting file %d, %v", ix, err)
		}
		err = c.CloseFd(fd)
		if err != nil {
			db.DFatalf("Run: failed to close fd for starting file %d, %v", ix, err)
		}
	}

	for ix := 0; ix < c.nWorkers; ix++ {
		go func (ix int) {
			p := proc.NewProc("watchperf-worker", []string{strconv.Itoa(ix), strconv.Itoa(c.nTrials), c.watchDir, c.responseDir, c.tempDir})
			err := c.Spawn(p)
			if err != nil {
				db.DFatalf("Run: spawning %d failed %v", ix, err)
			}
			err = c.WaitStart(p.GetPid())
			if err != nil {
				db.DFatalf("Run: starting %d failed %v", ix, err)
			}
			_, err = c.WaitExit(p.GetPid())
			if err != nil {
				db.DFatalf("Run: running %d failed %v", ix, err)
			}
		}(ix)
	}

	db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to signal they're ready")
	dirReader := fslib.NewDirReader(c.FsLib, c.responseDir)
	err := dirReader.WaitNEntries(c.nWorkers)
	if err != nil {
		db.DFatalf("Run: failed to wait for all procs to be ready %v", err)
	}
	c.clearResponseDir()

	creationDelays := make([][]time.Duration, c.nTrials)
	deletionDelays := make([][]time.Duration, c.nTrials)

	for trial := 0; trial < c.nTrials; trial++ {
		db.DPrintf(db.WATCH_PERF, "Run: Running trial %d", trial)
		path := filepath.Join(c.watchDir, fmt.Sprintf("trial_%d", trial))

		db.DPrintf(db.WATCH_PERF, "Run: Creating file for trial %d", trial)
		creationTime := time.Now()
		fd, err := c.Create(path, 0777, sigmap.OAPPEND)
		if err != nil {
			db.DFatalf("Run: failed to create trial file %d, %v", trial, err)
		}
		// wait for all children to recognize the creation
		// create a new dirReader because WaitNEntries caches responses and still has the old one as available
		db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to see creation for trial %d", trial)
		dirReader = fslib.NewDirReader(c.FsLib, c.responseDir)
		err = dirReader.WaitNEntries(c.nWorkers)
		if err != nil {
			db.DFatalf("Run: failed to wait for all procs to respond to creation during trial %d, %v", trial, err)
		}

		creationDelays[trial] = c.getWorkerDelays(creationTime)
		db.DPrintf(db.WATCH_PERF, "Run: Creation delays for trial %d: %v", trial, creationDelays[trial])
		c.clearResponseDir()

		db.DPrintf(db.WATCH_PERF, "Run: Removing file for trial %d", trial)
		deletionTime := time.Now()
		err = c.Remove(path)
		if err != nil {
			db.DFatalf("Run: failed to remove trial file %d, %v", trial, err)
		}
		// wait for all children to recognize the deletion
		db.DPrintf(db.WATCH_PERF, "Run: Waiting for workers to see deletion for trial %d", trial)
		dirReader = fslib.NewDirReader(c.FsLib, c.responseDir)
		err = dirReader.WaitNEntries(c.nWorkers)
		if err != nil {
			db.DFatalf("Run: failed to wait for all procs to respond to deletion during trial %d, %v", trial, err)
		}
		deletionDelays[trial] = c.getWorkerDelays(deletionTime)
		db.DPrintf(db.WATCH_PERF, "Run: Deletion delays for trial %d: %v", trial, deletionDelays[trial])
		c.clearResponseDir()

		err = c.CloseFd(fd)
		if err != nil {
			db.DFatalf("Run: failed to close trial file %d, %v", trial, err)
		}
	}

	for ix := 0; ix < c.nStartFiles; ix++ {
		path := filepath.Join(c.watchDir, strconv.Itoa(ix))
		err = c.Remove(path)
		if err != nil {
			db.DFatalf("Run: failed to remove %s, %v", path, err)
		}
	}

	if c.Remove(c.watchDir) != nil {
		db.DFatalf("Run: failed to remove watchdir %v", err)
	}
	if c.Remove(c.responseDir) != nil {
		db.DFatalf("Run: failed to remove responsedir %v", err)
	}
	if c.Remove(c.tempDir) != nil {
		db.DFatalf("Run: failed to remove tempdir %v", err)
	}
	if c.Remove(c.baseDir) != nil {
		db.DFatalf("Run: failed to remove basedir %v", err)
	}

	result := Result{
		CreationTimeNs: creationDelays,
		DeletionTimeNs: deletionDelays,
	}
	status := proc.NewStatusInfo(proc.StatusOK, "", result)
	err = c.ClntExit(status)
	if err != nil {
		db.DFatalf("Run: failed to exit client %v", err)
	}
}

func (c *Coord) clearResponseDir() {
	db.DPrintf(db.WATCH_PERF, "clearResponseDir: Clearing response dir")
	err := c.RmDirEntries(c.responseDir)
	if err != nil {
		db.DFatalf("clearResponseDir: failed to clear response dir entries %v", err)
	}
}

func (c *Coord) getWorkerDelays(startTime time.Time) []time.Duration {
	db.DPrintf(db.WATCH_PERF, "getWorkerDelays: Getting proc times")
	times := c.getWorkerTimes()
	db.DPrintf(db.WATCH_PERF, "worker times: %v", times)
	deltas := make([]time.Duration, c.nWorkers)
	for ix := 0; ix < c.nWorkers; ix++ {
		deltas[ix] = times[ix].Sub(startTime)
	}

	return deltas
}

func (c *Coord) getWorkerTimes() []time.Time {
	times := make([]time.Time, c.nWorkers)
	stats, _, err := c.ReadDir(c.responseDir)
	if err != nil {
		db.DFatalf("getWorkerTimes: failed to read response dir %v", err)
	}

	db.DPrintf(db.WATCH_PERF, "getWorkerTimes: Getting proc times")

	if len(stats) != c.nWorkers {
		db.DFatalf("getWorkerTimes: number of files in %s is not equal to num of workers %d, %v", c.responseDir, c.nWorkers, stats)
	}
	for _, stat := range stats {
		db.DPrintf(db.WATCH_PERF, "getWorkerTimes: Found file %s", stat.Name)
		id, err := strconv.Atoi(stat.Name)
		if err != nil {
			db.DFatalf("getWorkerTimes: file name could not be parsed as proc id: %s, %v", stat.Name, err)
		}

		path := filepath.Join(c.responseDir, stat.Name)
		reader, err := c.OpenReader(path)
		if err != nil {
			db.DFatalf("getWorkerTimes: failed to open file %s, %v", path, err)
		}
		scanner := bufio.NewScanner(reader.Reader)
		exists := scanner.Scan()
		if !exists {
			db.DFatalf("getWorkerTimes: file %s does not contain line", path)
		}

		text := scanner.Text()
		db.DPrintf(db.WATCH_PERF, "getWorkerTimes: File %s has contents %s", stat.Name, text)
		time, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			db.DFatalf("getWorkerTimes: failed to parse time %s, %v", text, err)
		}
		times[id] = time
	}

	return times
}
