package dirwatcher_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt/fslib/dirwatcher"
	"sigmaos/test"
	"sort"
	"strconv"
	"testing"
	"time"

	drtest "sigmaos/sigmaclnt/fslib/dirwatcher/test"
	sp "sigmaos/sigmap"
	protsrv_proto "sigmaos/spproto/srv/proto"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

func TestCompile(t *testing.T) {
}

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
	if !status.IsStatusOK() {
		assert.Fail(t, "coord did not return OK, err: %v", err)
	}

	ts.Shutdown()
}

type Stats struct {
	Average float64
	Max     int64
	Min     int64
	Stddev  float64
	Median  int64
}

func flatten(data [][][]time.Duration) []time.Duration {
	flat := make([]time.Duration, 0)
	for _, d := range data {
		for _, e := range d {
			flat = append(flat, e...)
		}
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
		stddev += math.Pow(float64(d.Nanoseconds())-average, 2)
	}
	stddev /= float64(len(data))
	stddev = math.Sqrt(stddev)

	sort.Slice(data, func(i, j int) bool {
		return data[i] < data[j]
	})
	median := data[len(data)/2].Nanoseconds()

	return Stats{average, maxT, minT, stddev, median}
}

func (s Stats) String() string {
	return fmt.Sprintf("Avg: %f us\nMax: %f us\nMin: %f us\nStddev: %f us\nMedian: %f us\n", s.Average/1000.0, float64(s.Max)/1000.0, float64(s.Min)/1000.0, s.Stddev/1000.0, float64(s.Median)/1000.0)
}

func dataString(data []time.Duration) string {
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%d,", d.Nanoseconds())
	}
	return str
}

func testPerf(t *testing.T, nWorkers int, nStartingFiles int, nTrials int, nFilesPerTrial int, useNamed bool) {
	ts, err := test.NewTstateAll(t)

	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	fmt.Printf("Running perf test with %d workers, %d starting files, %d trials, %d files per trial, useNamed %t\n",
		nWorkers, nStartingFiles, nTrials, nFilesPerTrial, useNamed)

	useNamedStr := "0"
	if useNamed {
		useNamedStr = "1"
	}
	p := proc.NewProc("watchperf-coord", []string{strconv.Itoa(nWorkers), strconv.Itoa(nStartingFiles), strconv.Itoa(nTrials), strconv.Itoa(nFilesPerTrial), useNamedStr})
	err = ts.Spawn(p)
	assert.Nil(t, err)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	if !status.IsStatusOK() {
		assert.Fail(t, "coord did not return OK, err: %v", err)
	}

	data := status.Data()
	result := drtest.Result{}
	mapstructure.Decode(data, &result)

	fmt.Printf("Creation Watch Delays:\n%s\n", computeStats(flatten(result.CreationWatchTimeNs)))

	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket != "" {
		storageType := "local"
		if useNamed {
			storageType = "named"
		}

		s3Folder := filepath.Join("name/s3/~any/", s3Bucket)
		if err := ts.MkDir(s3Folder, 0777); err != nil {
			if !serr.IsErrCode(err, serr.TErrExists) {
				assert.Fail(t, "Failed to create s3 folder: %v", err)
			}
		}

		if err := ts.MkDir(s3Folder, 0777); err != nil {
			if !serr.IsErrCode(err, serr.TErrExists) {
				assert.Fail(t, "Failed to create s3 folder: %v", err)
			}
		}

		filename := fmt.Sprintf("%dwkrs_%dstfi_%dfpt_%s", nWorkers, nStartingFiles, nFilesPerTrial, storageType)
		s3Filepath := filepath.Join(s3Folder, filename)
		fd, err := ts.Create(s3Filepath, 0777, sp.OWRITE)
		assert.Nil(t, err)

		creationWatchDelaysString := dataString(flatten(result.CreationWatchTimeNs))
		_, err = ts.Write(fd, []byte(creationWatchDelaysString))
		assert.Nil(t, err)
		ts.CloseFd(fd)
	}

	ts.Shutdown()
}

func TestSumProgramSingleWorker(t *testing.T) {
	testSumProgram(t, 1, 5)
}

func TestSumProgramMultipleWorkers(t *testing.T) {
	testSumProgram(t, 5, 100)
}

func TestSumProgramStress(t *testing.T) {
	testSumProgram(t, 10, 1000)
}

func TestPerf(t *testing.T) {
	useNamed := false
	numWorkers := 1
	numStartingFiles := 0
	numTrials := 250
	numFilesPerTrial := 1

	if os.Getenv("USE_NAMED") != "" {
		if os.Getenv("USE_NAMED") == "1" {
			useNamed = true
		} else if os.Getenv("USE_NAMED") == "0" {
			useNamed = false
		} else {
			assert.Fail(t, "Invalid value for USE_NAMED")
		}
	}

	if os.Getenv("NUM_WORKERS") != "" {
		var ok error
		numWorkers, ok = strconv.Atoi(os.Getenv("NUM_WORKERS"))
		if ok != nil {
			assert.Fail(t, "Invalid value for NUM_WORKERS")
		}
	}

	if os.Getenv("NUM_STARTING_FILES") != "" {
		var ok error
		numStartingFiles, ok = strconv.Atoi(os.Getenv("NUM_STARTING_FILES"))
		if ok != nil {
			assert.Fail(t, "Invalid value for NUM_STARTING_FILES")
		}
	}

	if os.Getenv("NUM_FILES_PER_TRIAL") != "" {
		var ok error
		numFilesPerTrial, ok = strconv.Atoi(os.Getenv("NUM_FILES_PER_TRIAL"))
		if ok != nil {
			assert.Fail(t, "Invalid value for NUM_FILES_PER_TRIAL")
		}
	}

	if os.Getenv("NUM_TRIALS") != "" {
		var ok error
		numTrials, ok = strconv.Atoi(os.Getenv("NUM_TRIALS"))
		if ok != nil {
			assert.Fail(t, "Invalid value for NUM_TRIALS")
		}
	}

	testPerf(t, numWorkers, numStartingFiles, numTrials, numFilesPerTrial, useNamed)
}

func runTest(t *testing.T, f func(*testing.T, *test.Tstate, string), timeoutSec int) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err, "Error New Tstate: %v", err)

	done := make(chan bool)
	timeout := time.After(time.Duration(timeoutSec) * time.Second)
	go func() {
		testdir := filepath.Join(sp.NAMED, "test")
		err = ts.MkDir(testdir, 0777)
		assert.Nil(t, err)

		f(t, ts, testdir)

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

func TestDirWatcherChannel(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string) {
		fd, err := ts.Open(testdir, sp.OREAD)
		assert.Nil(t, err)
		dw, err := dirwatcher.NewDirWatcher(ts.FsLib, testdir, fd)
		assert.Nil(t, err)

		ch := dw.Events()
		received := make(chan struct{})
		file1 := "file1"

		go func() {
			event1 := <-ch
			assert.Equal(t, event1.File, file1)
			assert.Equal(t, event1.Type, protsrv_proto.WatchEventType_CREATE)
			received <- struct{}{}

			event2 := <-ch
			assert.Equal(t, event2.File, file1)
			assert.Equal(t, event2.Type, protsrv_proto.WatchEventType_REMOVE)
			received <- struct{}{}
		}()

		_, err = ts.Create(filepath.Join(testdir, file1), 0777, sp.OWRITE)
		assert.Nil(t, err)
		<-received

		err = ts.Remove(filepath.Join(testdir, file1))
		assert.Nil(t, err)
		<-received

		err = dw.Close()
		assert.Nil(t, err)
		err = ts.CloseFd(fd)
		assert.Nil(t, err)
	}, 10)
}

func TestDirWatcherNEntries(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string) {
		err := dirwatcher.WaitNEntries(ts.FsLib, testdir, 0)
		assert.Nil(t, err)

		go func() {
			time.Sleep(10 * time.Millisecond)
			_, err = ts.Create(filepath.Join(testdir, "file0"), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}()

		err = dirwatcher.WaitNEntries(ts.FsLib, testdir, 0)
		assert.Nil(t, err)

		done := make(chan bool)
		go func() {
			err = dirwatcher.WaitNEntries(ts.FsLib, testdir, 10)
			assert.Nil(t, err)
			done <- true
		}()

		for i := 1; i < 10; i++ {
			_, err = ts.Create(filepath.Join(testdir, fmt.Sprintf("file%d", i)), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		<-done
	}, 10)
}

func TestDirWatcherWaitEmpty(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, testdir string) {
		err := dirwatcher.WaitEmpty(ts.FsLib, testdir)
		assert.Nil(t, err)

		for ix := 0; ix < 10; ix++ {
			_, err = ts.Create(filepath.Join(testdir, fmt.Sprintf("file%d", ix)), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		err = dirwatcher.WaitNEntries(ts.FsLib, testdir, 10)
		assert.Nil(t, err)

		done := make(chan bool)
		go func() {
			err = dirwatcher.WaitEmpty(ts.FsLib, testdir)
			assert.Nil(t, err)
			done <- true
		}()

		for ix := 0; ix < 10; ix++ {
			err = ts.Remove(filepath.Join(testdir, fmt.Sprintf("file%d", ix)))
			assert.Nil(t, err)
		}

		<-done
	}, 10)
}
