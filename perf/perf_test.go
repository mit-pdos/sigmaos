package perf_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/perf"
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

	// Start a spinning thread to consume a core.
	go spin(done)

	// Wait for the spinning thread to start
	time.Sleep(100 * time.Millisecond)

	utime0, stime0, utime1, stime1 = tick(pid, &t0)
	util = perf.UtilFromCPUTimeSample(utime0, stime0, utime1, stime1, time.Since(t0).Seconds())

	db.DPrintf("TEST", "Util (1 spinner): %v", util)

	assert.True(t, util >= 95.0, "Util too low: %v", util)
	assert.True(t, util < 105.0, "Util too high: %v", util)

	// Start another spinning thread to consume a core.
	go spin(done)

	// Wait for the spinning thread to start
	time.Sleep(100 * time.Millisecond)

	utime0, stime0, utime1, stime1 = tick(pid, &t0)
	util = perf.UtilFromCPUTimeSample(utime0, stime0, utime1, stime1, time.Since(t0).Seconds())

	db.DPrintf("TEST", "Util (2 spinners): %v", util)

	assert.True(t, util >= 195.0, "Util too low: %v", util)
	assert.True(t, util < 205.0, "Util too high: %v", util)

	// Start yet another spinning thread to consume a core.
	go spin(done)

	// Wait for the spinning thread to start
	time.Sleep(100 * time.Millisecond)

	utime0, stime0, utime1, stime1 = tick(pid, &t0)
	util = perf.UtilFromCPUTimeSample(utime0, stime0, utime1, stime1, time.Since(t0).Seconds())

	db.DPrintf("TEST", "Util (3 spinners): %v", util)

	assert.True(t, util >= 295.0, "Util too low: %v", util)
	assert.True(t, util < 305.0, "Util too high: %v", util)

	done <- true
	done <- true
}
