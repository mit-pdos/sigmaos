package watch_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sigmaos/proc"
	"sigmaos/test"
	"strconv"
	"strings"
	"testing"
	"time"

	sp "sigmaos/sigmap"
	"sigmaos/watch"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

func testWatch(t *testing.T, nworkers int, nfiles int) {
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

	return Stats{average, maxT, minT, stddev}
}

func (s Stats) String() string {
	return fmt.Sprintf("Avg: %f us\nMax: %f us\nMin: %f us\nStddev: %f us", s.Average / 1000.0, float64(s.Max) / 1000.0, float64(s.Min) / 1000.0, s.Stddev / 1000.0)
}

func dataString(data []time.Duration) string {
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%d,", d.Nanoseconds())
	}
	return str
}

func testWatchPerf(t *testing.T, nWorkers int, nStartingFiles int, nTrials int, prefix string) {
	ts, err := test.NewTstateAll(t)

	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	basedir := filepath.Join(sp.NAMED, "watchperf")

	measureMode := os.Getenv("WATCHPERF_MEASURE_MODE")
	if measureMode == "" {
		measureMode = strconv.Itoa(int(watch.JustWatch))
	}

	fmt.Printf("Using measure mode %s\n", measureMode)

	p := proc.NewProc("watchperf-coord", []string{strconv.Itoa(nWorkers), strconv.Itoa(nStartingFiles), strconv.Itoa(nTrials), basedir, measureMode})
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
	result := watch.Result{}
	mapstructure.Decode(data, &result)

	fmt.Printf("Creation Watch Delays:\n%s\n", computeStats(flatten(result.CreationWatchTimeNs)))
	fmt.Printf("Deletion Watch Delays:\n%s\n", computeStats(flatten(result.DeletionWatchTimeNs)))

	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket != "" {
		s3Filepath := filepath.Join("name/s3/~any/" + os.Getenv(s3Bucket), fmt.Sprintf("watchperf_%s_%s.txt", prefix, time.Now().String()))
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

func TestWatchSingle(t *testing.T) {
	testWatch(t, 1, 5)
}

func TestWatchMultiple(t *testing.T) {
	testWatch(t, 5, 50)
}

func TestWatchStress(t *testing.T) {
	testWatch(t, 10, 1000)
}

// Use USE_OLD_WATCH and WATCHPERF_MEASURE_MODE to configure perf data
func TestWatchPerfSingleWorkerNoFiles(t *testing.T) {
	testWatchPerf(t, 1, 0, 250, "single_no_files")
}

func TestWatchPerfSingleWorkerSomeFiles(t *testing.T) {
	testWatchPerf(t, 1, 100, 250, "single_some_files")
}

func TestWatchPerfSingleWorkerManyFiles(t *testing.T) {
	testWatchPerf(t, 1, 1000, 250, "single_many_files")
}

func TestWatchPerfMultipleWorkersNoFiles(t *testing.T) {
	testWatchPerf(t, 5, 0, 100, "multiple_no_files")
}