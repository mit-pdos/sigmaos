package dirreader_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/test"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"sigmaos/fslib/dirreader"
	drtest "sigmaos/fslib/dirreader/test"
	sp "sigmaos/sigmap"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

func testSumProgram(t *testing.T, nworkers int, nfiles int) {
	ts, err := test.NewTstateAll(t)

	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	basedir := filepath.Join(sp.NAMED, "watchtest")

	p := proc.NewProc("watchtest-coord", []string{strconv.Itoa(nworkers), strconv.Itoa(nfiles), basedir})
	err = ts.Spawn(p)
	assert.Nil(t, err)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	if (!status.IsStatusOK()) {
		assert.Fail(t, "coord did not return OK, err: %v", err)
	}

	ts.Shutdown()
}

type Stats struct {
	Average float64
	Max int64
	Min int64
	Stddev float64
	Median int64
}

func flatten(data [][]time.Duration) []time.Duration {
	flat := make([]time.Duration, 0)
	for _, d := range data {
		flat = append(flat, d...)
	}
	return flat
}

func computeStats(data []time.Duration) Stats {
	average := float64(0)
	minT := data[0].Nanoseconds()
	maxT := data[0].Nanoseconds()
	for _, d := range data {
		average += float64(d.Nanoseconds())
		minT = min(minT, d.Nanoseconds())
		maxT = max(maxT, d.Nanoseconds())
	}
	average /= float64(len(data))

	stddev := float64(0)
	for _, d := range data {
		stddev += math.Pow(float64(d.Nanoseconds()) - average, 2)
	}
	stddev /= float64(len(data))
	stddev = math.Sqrt(stddev)

	sort.Slice(data, func (i, j int) bool {
		return data[i] < data[j]
	})
	median := data[len(data) / 2].Nanoseconds()

	return Stats{average, maxT, minT, stddev, median}
}

func (s Stats) String() string {
	return fmt.Sprintf("Avg: %f us\nMax: %f us\nMin: %f us\nStddev: %f us\nMedian %f\n", s.Average / 1000.0, float64(s.Max) / 1000.0, float64(s.Min) / 1000.0, s.Stddev / 1000.0, float64(s.Median) / 1000.0)
}

func dataString(data []time.Duration) string {
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%d,", d.Nanoseconds())
	}
	return str
}

func testPerf(t *testing.T, nWorkers int, nStartingFiles int, nTrials int, prefix string, useNamed bool, measureMode drtest.MeasureMode) {
	ts, err := test.NewTstateAll(t)

	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	
	var basedir string
	if useNamed {
		basedir = filepath.Join(sp.NAMED, "watchperf")
	} else {
		basedir = filepath.Join(sp.UX, sp.LOCAL, "watchperf")
	}

	measureModeStr := strconv.Itoa(int(measureMode))

	p := proc.NewProc("watchperf-coord", []string{strconv.Itoa(nWorkers), strconv.Itoa(nStartingFiles), strconv.Itoa(nTrials), basedir, measureModeStr})
	err = ts.Spawn(p)
	assert.Nil(t, err)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	if (!status.IsStatusOK()) {
		assert.Fail(t, "coord did not return OK, err: %v", err)
	}

	data := status.Data()
	result := drtest.Result{}
	mapstructure.Decode(data, &result)

	fmt.Printf("Creation Watch Delays:\n%s\n", computeStats(flatten(result.CreationWatchTimeNs)))
	fmt.Printf("Deletion Watch Delays:\n%s\n", computeStats(flatten(result.DeletionWatchTimeNs)))

	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket != "" {
		storageType := "local"
		if useNamed {
			storageType = "named"
		}

		measureModeString := "watch_only"
		if measureMode == drtest.IncludeFileOp {
			measureModeString = "include_op"
		}

		s3Folder := filepath.Join("name/s3/~any/", s3Bucket)
		if err := ts.MkDir(s3Folder, 0777); err != nil {
			if !serr.IsErrCode(err, serr.TErrExists) {
				assert.Fail(t, "Failed to create s3 folder: %v", err)
			}
		}

		versionFolder := "v" + strconv.Itoa(int(dirreader.GetDirReaderVersion(ts.ProcEnv())))
		s3FolderVersioned := filepath.Join(s3Folder, versionFolder)
		if err := ts.MkDir(s3FolderVersioned, 0777); err != nil {
			if !serr.IsErrCode(err, serr.TErrExists) {
				assert.Fail(t, "Failed to create s3 folder: %v", err)
			}
		}

		s3Filepath := filepath.Join(s3FolderVersioned, fmt.Sprintf("watchperf_%s_%s_%s.txt", prefix, storageType, measureModeString))
		fd, err := ts.Create(s3Filepath, 0777, sp.OWRITE)
		assert.Nil(t, err)

		creationWatchDelaysString := dataString(flatten(result.CreationWatchTimeNs))
		deletionWatchDelaysString := dataString(flatten(result.DeletionWatchTimeNs))
		writeString := strings.Join([]string{
			creationWatchDelaysString,
			deletionWatchDelaysString}, "\n")
		_, err = ts.Write(fd, []byte(writeString))
		assert.Nil(t, err)
		ts.CloseFd(fd)
	}

	ts.Shutdown()
}

func TestSumProgramSingleWorker(t *testing.T) {
	testSumProgram(t, 1, 5)
}

func TestSumProgramMultipleWorkers(t *testing.T) {
	testSumProgram(t, 5, 50)
}

func TestSumProgramStress(t *testing.T) {
	testSumProgram(t, 10, 1000)
}

func runTestForAll(f func(bool, drtest.MeasureMode)) {
	measureModeOptions := []drtest.MeasureMode{drtest.IncludeFileOp, drtest.JustWatch}
	useNamedOptions := []bool{false, true}
	dirreaderOptions := []dirreader.DirReaderVersion{dirreader.V1, dirreader.V2}

	if os.Getenv("MEASURE_MODE") != "" {
		if os.Getenv("MEASURE_MODE") == "watch_only" {
			measureModeOptions = []drtest.MeasureMode{drtest.JustWatch}
		} else if os.Getenv("MEASURE_MODE") == "include_op" {
			measureModeOptions = []drtest.MeasureMode{drtest.IncludeFileOp}
		} else {
			panic("Invalid MEASURE_MODE")
		}
	}

	if os.Getenv("USE_NAMED") != "" {
		if os.Getenv("USE_NAMED") == "1" {
			useNamedOptions = []bool{true}
		} else if os.Getenv("USE_NAMED") == "0" {
			useNamedOptions = []bool{false}
		} else {
			panic("Invalid USE_NAMED")
		}
	}

	if os.Getenv("DIRREADER_VERSION") != "" {
		if os.Getenv("DIRREADER_VERSION") == "1" {
			dirreaderOptions = []dirreader.DirReaderVersion{dirreader.V1}
		} else if os.Getenv("DIRREADER_VERSION") == "2" {
			dirreaderOptions = []dirreader.DirReaderVersion{dirreader.V2}
		} else {
			panic("Invalid DIRREADER_VERSION")
		}
	}

	for _, measureMode := range measureModeOptions {
		for _, useNamed := range useNamedOptions {
			for _, dirreaderVersion := range dirreaderOptions {
				fmt.Println("Running test with useNamed:", useNamed, "measureMode:", measureMode, "dirreaderVersion:", dirreaderVersion)
				os.Setenv("DIRREADER_VERSION", strconv.Itoa(int(dirreaderVersion)))
				f(useNamed, measureMode)
			}
		}
	}
}

// Use DIRREADER_VERSION to configure perf data
func TestPerfSingleWorkerNoFiles(t *testing.T) {
	runTestForAll(func(useNamed bool, measureMode drtest.MeasureMode) {
		testPerf(t, 1, 0, 250, "single_no_files", useNamed, measureMode)
	})
}

func TestPerfSingleWorkerSomeFiles(t *testing.T) {
	runTestForAll(func(useNamed bool, measureMode drtest.MeasureMode) {
		testPerf(t, 1, 100, 250, "single_some_files", useNamed, measureMode)
	})
}

func TestPerfSingleWorkerManyFiles(t *testing.T) {
	runTestForAll(func(useNamed bool, measureMode drtest.MeasureMode) {
		testPerf(t, 1, 1000, 250, "single_many_files", useNamed, measureMode)
	})
}

func TestPerfMultipleWorkersNoFiles(t *testing.T) {
	runTestForAll(func(useNamed bool, measureMode drtest.MeasureMode) {
		testPerf(t, 5, 0, 100, "multiple_no_files", useNamed, measureMode)
	})
}

func runTest(t *testing.T, f func(*testing.T, *test.Tstate, string, dirreader.DirReader), timeoutSec int) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err, "Error New Tstate: %v", err)

	done := make(chan bool)
	timeout := time.After(time.Duration(timeoutSec) * time.Second)
	go func() {
		testdir := filepath.Join(sp.NAMED, "test")
		err = ts.MkDir(testdir, 0777)
		assert.Nil(t, err)

		dr, err := dirreader.NewDirReader(ts.FsLib, testdir)
		assert.Nil(t, err)

		f(t, ts, testdir, dr)

		err = dr.Close()
		assert.Nil(t, err)

		err = ts.RmDirEntries(testdir)
		assert.Nil(t, err)

		err = ts.Remove(testdir)
		assert.Nil(t, err)

		done <- true
	}()

	select {
	case <-timeout:
		assert.Fail(t, "Timeout")
	case <-done:
	}

	ts.Shutdown()
}

func TestDirReaderBasic(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string, dr dirreader.DirReader) {
		entries, err := dr.GetDir()
		assert.Nil(t, err)
		assert.Equal(t, len(entries), 0)

		file1 := "file1"
		_, err = ts.Create(filepath.Join(testdir, file1), 0777, sp.OWRITE)
		assert.Nil(t, err)

		err = dr.WaitCreate(file1)
		assert.Nil(t, err, "")

		entries, err = dr.GetDir()
		assert.Nil(t, err)
		assert.Equal(t, entries, []string{file1})

		err = ts.Remove(filepath.Join(testdir, file1))
		assert.Nil(t, err)

		err = dr.WaitRemove(file1)
		assert.Nil(t, err)

		entries, err = dr.GetDir()
		assert.Nil(t, err)
		assert.Equal(t, len(entries), 0)
	}, 10)
}

func TestDirReaderWaitNEntries(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string, dr dirreader.DirReader) {
		err := dr.WaitNEntries(0)
		assert.Nil(t, err)

		_, err = ts.Create(filepath.Join(testdir, "file0"), 0777, sp.OWRITE)
		assert.Nil(t, err)

		err = dr.WaitNEntries(1)
		assert.Nil(t, err)

		done := make(chan bool)
		go func() {
			err := dr.WaitNEntries(10)
			assert.Nil(t, err)
			done <- true
		}()

		for i := 1; i < 10; i++ {
			_, err = ts.Create(filepath.Join(testdir, fmt.Sprintf("file%d", i)), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		<- done
	}, 10)
}

func TestDirReaderWaitEmpty(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string, dr dirreader.DirReader) {
		err := dr.WaitEmpty()
		assert.Nil(t, err)

		for ix := 0; ix < 10; ix++ {
			_, err = ts.Create(filepath.Join(testdir, fmt.Sprintf("file%d", ix)), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		err = dr.WaitNEntries(10)
		assert.Nil(t, err)

		done := make(chan bool)
		go func() {
			err := dr.WaitEmpty()
			assert.Nil(t, err)
			done <- true
		}()

		for ix := 0; ix < 10; ix++ {
			err = ts.Remove(filepath.Join(testdir, fmt.Sprintf("file%d", ix)))
			assert.Nil(t, err)
		}

		<- done
	}, 10)
}

func TestDirReaderWatchEntriesChangedRelative(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string, dr dirreader.DirReader) {
		for _, file := range []string{"a", "b", "c"} {
			_, err := ts.Create(filepath.Join(testdir, file), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		entries, ok, err := dr.WatchEntriesChangedRelative([]string{}, []string{})
		assert.True(t, ok)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, entries)

		for _, file := range []string{"bb", "cc", "dd"} {
			_, err := ts.Create(filepath.Join(testdir, file), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}
		time.Sleep(1 * time.Second)
		assert.Nil(t, err)

		entries, ok, err = dr.WatchEntriesChangedRelative([]string{"a", "b", "c"}, []string{"b"})
		assert.True(t, ok)
		assert.Nil(t, err)
		assert.Equal(t, []string{"cc", "dd"}, entries)

		done := make(chan bool)
		go func() {
			entries, ok, err = dr.WatchEntriesChangedRelative([]string{"a", "b", "bb", "c", "cc", "dd", "eee"}, []string{"b"})
			assert.True(t, ok)
			assert.Nil(t, err)
			assert.Contains(t, entries, "fff") // could contain or not contain eee depending on whether changes were grouped or not
			done <- true
		}()

		_, err = ts.Create(filepath.Join(testdir, "eee"), 0777, sp.OWRITE)
		assert.Nil(t, err)
		_, err = ts.Create(filepath.Join(testdir, "fff"), 0777, sp.OWRITE)
		assert.Nil(t, err)

		<- done
	}, 10)
}

func TestDirReaderWatchEntriesChanged(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string, dr dirreader.DirReader) {
		initialFiles := []string{"file1", "file2", "file3"}
		for _, file := range initialFiles {
			_, err := ts.Create(filepath.Join(testdir, file), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		time.Sleep(1 * time.Second)

		changes, err := dr.WatchEntriesChanged()
		assert.Nil(t, err)
		assert.Equal(t, 3, len(changes))
		for _, file := range initialFiles {
			assert.True(t, changes[file])
		}

		done := make(chan bool)
		go func() {
			changes, err = dr.WatchEntriesChanged()
			assert.Nil(t, err)
			assert.True(t, changes["file4"])
			assert.Equal(t, 1, len(changes))
			done <- true
		}()

		_, err = ts.Create(filepath.Join(testdir, "file4"), 0777, sp.OWRITE)
		assert.Nil(t, err)

		<-done
	}, 10)
}

func TestDirReaderWatchNewEntriesAndRename(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string, dr dirreader.DirReader) {
		dstDir := filepath.Join(sp.NAMED, "dst")
		err := ts.MkDir(dstDir, 0777)
		assert.Nil(t, err)

		initialFiles := []string{"file1", "file2"}
		for _, file := range initialFiles {
			_, err := ts.Create(filepath.Join(testdir, file), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		movedFiles, err := dr.WatchNewEntriesAndRename(dstDir)
		assert.Nil(t, err)
		assert.ElementsMatch(t, movedFiles, initialFiles)

		entries, _, err := ts.ReadDir(dstDir)
		assert.Nil(t, err)
		for _, file := range initialFiles {
			found := false
			for _, entry := range entries {
				if entry.Name == file {
					found = true
					break
				}
			}
			assert.True(t, found)
		}

		done := make(chan bool)
		go func() {
			movedFiles, err = dr.WatchNewEntriesAndRename(dstDir)
			assert.Nil(t, err)
			assert.ElementsMatch(t, movedFiles, []string{"file3"})
			done <- true
		}()

		_, err = ts.Create(filepath.Join(testdir, "file3"), 0777, sp.OWRITE)
		assert.Nil(t, err)

		<-done

		// cleanup
		err = ts.RmDirEntries(dstDir)
		assert.Nil(t, err)

		err = ts.Remove(dstDir)
		assert.Nil(t, err)
	}, 10)
}

func TestDirReaderGetEntriesAndRename(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string, dr dirreader.DirReader) {
		dstDir := filepath.Join(sp.NAMED, "dst")
		err := ts.MkDir(dstDir, 0777)
		assert.Nil(t, err)

		initialFiles := []string{"file1", "file2"}
		for _, file := range initialFiles {
			_, err := ts.Create(filepath.Join(testdir, file), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		// for V2, we need to wait for the files to be up to date in the cache,
		// but doing this in V1 will cause V1 to no longer pick up on the files
		if dirreader.GetDirReaderVersion(ts.ProcEnv()) == dirreader.V1 {
			time.Sleep(1 * time.Second)
		} else {
			err = dr.WaitNEntries(2)
			assert.Nil(t, err)
		}

		movedFiles, err := dr.GetEntriesAndRename(dstDir)
		assert.Nil(t, err)
		assert.ElementsMatch(t, movedFiles, initialFiles)

		entries, _, err := ts.ReadDir(dstDir)
		assert.Nil(t, err)
		for _, file := range initialFiles {
			found := false
			for _, entry := range entries {
				if entry.Name == file {
					found = true
					break
				}
			}
			assert.True(t, found)
		}

		// ensure this doesn't block
		movedFiles, err = dr.GetEntriesAndRename(dstDir)
		assert.Nil(t, err)
		assert.Empty(t, movedFiles)

		// Ensure no entries remain in the original directory
		remainingEntries, err := dr.GetDir()
		assert.Nil(t, err)
		assert.Empty(t, remainingEntries)

		// cleanup
		err = ts.RmDirEntries(dstDir)
		assert.Nil(t, err)

		err = ts.Remove(dstDir)
		assert.Nil(t, err)
	}, 10)
}