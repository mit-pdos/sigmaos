package perf_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/perf"
)

func TestGetSamples(t *testing.T) {
	hz := perf.Hz()
	assert.NotEqual(t, 0, hz, "Hz")

	cores := perf.GetActiveCores()
	idle1, total1 := perf.GetCPUSample(cores)
	assert.NotEqual(t, 0, idle1, "GetCPUSample")
	assert.True(t, idle1 < total1, "total")
}

// Spin a lot in order to consume a core fully.
func spin(done chan bool) {
	for {
		select {
		case <-done:
			break
		default:
		}
	}
}

func tick(pid string, t0 *time.Time) (utime0, stime0, utime1, stime1 uint64) {
	*t0 = time.Now()
	utime0, stime0 = perf.GetCPUTimePid(pid)
	time.Sleep(100 * time.Millisecond)
	utime1, stime1 = perf.GetCPUTimePid(pid)
	return
}

func TestGetCPUTimePid(t *testing.T) {
	done := make(chan bool)
	pid := strconv.Itoa(os.Getpid())

	var utime0 uint64
	var utime1 uint64
	var stime0 uint64
	var stime1 uint64
	var util float64
	var t0 time.Time

	utime0, stime0, utime1, stime1 = tick(pid, &t0)
	util = perf.UtilFromCPUTimeSample(utime0, stime0, utime1, stime1, time.Since(t0).Seconds())

	db.DPrintf("TEST", "Util (sleep): %v", util)

	assert.True(t, util >= 0.0, "Util negative: %v", util)
	assert.True(t, util < 5.0, "Util too high: %v", util)

	const N = 3
	for i := 0; i < N; i++ {
		// Start a spinning thread to consume a core.
		go spin(done)

		// Wait for the spinning thread to start
		time.Sleep(100 * time.Millisecond)

		utime0, stime0, utime1, stime1 = tick(pid, &t0)
		util = perf.UtilFromCPUTimeSample(utime0, stime0, utime1, stime1, time.Since(t0).Seconds())

		db.DPrintf("TEST", "Util (%v spinner): %v", i, util)

		assert.True(t, util >= 100.0*float64(i+1)-10.0, "Util too low (i=%v): %v", i, util)
		assert.True(t, util < 100.0*float64(i+1)+10.0, "Util too high (i=%v): %v", i, util)
	}

	for i := 0; i < N; i++ {
		done <- true
	}
}
