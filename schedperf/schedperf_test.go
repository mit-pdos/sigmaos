package schedperf_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	"sigmaos/scheddclnt"
	"sigmaos/test"
)

const (
	N_ITER      = 5_000_000_000
	SLEEP_MSECS = 8000

	UTIL_READ_FREQ_MS = 10

	REALM1 = "testrealm"
)

func spawnSpinPerf(ts *test.RealmTstate, mcpu proc.Tmcpu, nthread uint, niter int, id string) proc.Tpid {
	p := proc.MakeProc("spinperf", []string{"true", strconv.Itoa(int(nthread)), strconv.Itoa(niter), id})
	p.SetMcpu(mcpu)
	err := ts.Spawn(p)
	assert.Nil(ts.T, err, "Error spawn: %v", err)
	return p.GetPid()
}

func spawnSleeper(ts *test.RealmTstate) proc.Tpid {
	p := proc.MakeProc("sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/"})
	err := ts.Spawn(p)
	assert.Nil(ts.T, err, "Error spawn: %v", err)
	return p.GetPid()
}

func wait(ts *test.RealmTstate, pid proc.Tpid) time.Duration {
	status, err := ts.WaitExit(pid)
	assert.Nil(ts.T, err, "WaitExit error")
	assert.True(ts.T, status.IsStatusOK(), "Exit status wrong: %v", status)
	return time.Duration(status.Data().(float64))
}

func calibrateCTimeSigma(ts *test.RealmTstate, nthread uint, niter int) time.Duration {
	c := make(chan time.Duration)
	go runSpinPerf(ts, c, 0, nthread, niter, "sigma-baseline")
	return <-c
}

func runSpinPerf(ts *test.RealmTstate, c chan time.Duration, mcpu proc.Tmcpu, nthread uint, niter int, id string) {
	pid := spawnSpinPerf(ts, mcpu, nthread, niter, id)
	c <- wait(ts, pid)
}

func runSleeper(ts *test.RealmTstate, c chan time.Duration) {
	pid := spawnSleeper(ts)
	c <- wait(ts, pid)
}

func TestGetCPUUtilLatencyLowLoad(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	sdc := scheddclnt.MakeScheddClnt(ts1.SigmaClnt, ts1.GetRealm())

	db.DPrintf(db.TEST, "Run SpinPerf")
	c := make(chan time.Duration)
	go runSleeper(ts1, c)

	var done bool
	for !done {
		select {
		case d1 := <-c:
			db.DPrintf(db.TEST, "App latency: %v", d1)
			done = true
		default:
			start := time.Now()
			perc, err := sdc.GetCPUUtil()
			// Calculate latency of GetCPUUtil RPC.
			db.DPrintf(db.TEST, "GetCPUUtil lat:%v util:%v cores", time.Since(start), perc/100.0)
			assert.Nil(rootts.T, err, "Error get CPU util: %v", err)
		}
		time.Sleep(UTIL_READ_FREQ_MS * time.Millisecond)
	}

	rootts.Shutdown()
}

func TestGetCPUUtilLatencyHighLoad(t *testing.T) {
	rootts := test.MakeTstateWithRealms(t)
	ts1 := test.MakeRealmTstate(rootts, REALM1)

	sdc := scheddclnt.MakeScheddClnt(ts1.SigmaClnt, ts1.GetRealm())

	db.DPrintf(db.TEST, "Run SpinPerf")
	c := make(chan time.Duration)
	go runSpinPerf(ts1, c, 0, linuxsched.NCores-2, N_ITER, "spin1")

	var done bool
	for !done {
		select {
		case d1 := <-c:
			db.DPrintf(db.TEST, "App latency: %v", d1)
			done = true
		default:
			start := time.Now()
			perc, err := sdc.GetCPUUtil()
			// Calculate latency of GetCPUUtil RPC.
			db.DPrintf(db.TEST, "GetCPUUtil lat:%v util:%v cores", time.Since(start), perc/100.0)
			assert.Nil(rootts.T, err, "Error get CPU util: %v", err)
		}
		time.Sleep(UTIL_READ_FREQ_MS * time.Millisecond)
	}

	rootts.Shutdown()
}
