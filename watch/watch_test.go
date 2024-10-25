package watch_test

import (
	"fmt"
	"math"
	"path/filepath"
	"sigmaos/proc"
	"sigmaos/test"
	"strconv"
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

func testWatchPerf(t *testing.T, nWorkers int, nStartingFiles int, nTrials int, prefix string) {
	ts, err := test.NewTstateAll(t)

	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}

	basedir := filepath.Join(sp.NAMED, "watchperf")

	p := proc.NewProc("watchperf-coord", []string{strconv.Itoa(nWorkers), strconv.Itoa(nStartingFiles), strconv.Itoa(nTrials), basedir})
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

	creationDelays := result.CreationTimeNs
	deletionDelays := result.DeletionTimeNs

	// TODO make this code nicer by wrapping into a function and also calculate things like stddev
	averageCreationDelay := float64(0)
	minCreationDelay := creationDelays[0][0].Nanoseconds()
	maxCreationDelay := creationDelays[0][0].Nanoseconds()
	for trial := 0; trial < nTrials; trial++ {
		for worker := 0; worker < nWorkers; worker++ {
			averageCreationDelay += float64(creationDelays[trial][worker].Nanoseconds())
			minCreationDelay = min(minCreationDelay, creationDelays[trial][worker].Nanoseconds())
			maxCreationDelay = max(maxCreationDelay, creationDelays[trial][worker].Nanoseconds())
		}
	}
	averageCreationDelay /= float64(nTrials * nWorkers)

	averageDeletionDelay := float64(0)
	minDeletionDelay := deletionDelays[0][0].Nanoseconds()
	maxDeletionDelay := deletionDelays[0][0].Nanoseconds()
	for trial := 0; trial < nTrials; trial++ {
		for worker := 0; worker < nWorkers; worker++ {
			averageDeletionDelay += float64(deletionDelays[trial][worker].Nanoseconds())
			minDeletionDelay = min(minDeletionDelay, deletionDelays[trial][worker].Nanoseconds())
			maxDeletionDelay = max(maxDeletionDelay, deletionDelays[trial][worker].Nanoseconds())
		}
	}
	averageDeletionDelay /= float64(nTrials * nWorkers)

	// calculate stddev
	creationStddev := float64(0)
	for trial := 0; trial < nTrials; trial++ {
		for worker := 0; worker < nWorkers; worker++ {
			creationStddev += math.Pow(float64(creationDelays[trial][worker].Nanoseconds()) - averageCreationDelay, 2)
		}
	}
	creationStddev /= float64(nTrials * nWorkers)
	creationStddev = math.Sqrt(creationStddev)

	deletionStddev := float64(0)
	for trial := 0; trial < nTrials; trial++ {
		for worker := 0; worker < nWorkers; worker++ {
			deletionStddev += math.Pow(float64(deletionDelays[trial][worker].Nanoseconds()) - averageDeletionDelay, 2)
		}
	}
	deletionStddev /= float64(nTrials * nWorkers)
	deletionStddev = math.Sqrt(deletionStddev)

	fmt.Printf("Creation Delays:\n  Avg: %f us\n  Max: %f us\n  Min: %f us\n  Stddev: %f us\n", averageCreationDelay / 1000.0, float64(maxCreationDelay) / 1000.0, float64(minCreationDelay) / 1000.0, creationStddev / 1000.0)
	fmt.Printf("Deletion Delays:\n  Avg: %f us\n  Max: %f us\n  Min: %f us\n  Stddev: %f us\n", averageDeletionDelay / 1000.0, float64(maxDeletionDelay) / 1000.0, float64(minDeletionDelay) / 1000.0, deletionStddev / 1000.0)

	s3Filepath := filepath.Join("name/s3/~any/sigmaos-bucket-ryan/", fmt.Sprintf("watchperf_%s_%s.txt", prefix, time.Now().String()))
	fd, err := ts.Create(s3Filepath, 0777, sp.OWRITE)
	assert.Nil(t, err)

	creationDelaysString := ""
	deletionDelaysString := ""
	for trial := 0; trial < nTrials; trial++ {
		for worker := 0; worker < nWorkers; worker++ {
			creationDelaysString += strconv.FormatInt(creationDelays[trial][worker].Nanoseconds(), 10) + ","
			deletionDelaysString += strconv.FormatInt(deletionDelays[trial][worker].Nanoseconds(), 10) + ","
		}
	}
	writeString := creationDelaysString + "\n" + deletionDelaysString + "\n"
	_, err = ts.Write(fd, []byte(writeString))
	assert.Nil(t, err)
	ts.CloseFd(fd)

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