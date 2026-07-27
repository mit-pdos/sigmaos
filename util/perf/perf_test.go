package perf_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/perf"
)

func TestCompile(t *testing.T) {
	assert.NotNil(t, test.User)
}

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

func tick(pid string, t0 *time.Time) (utime0, stime0, utime1, stime1 uint64, err error) {
	*t0 = time.Now()
	utime0, stime0, err = perf.GetCPUTimePid(pid)
	time.Sleep(100 * time.Millisecond)
	utime1, stime1, err = perf.GetCPUTimePid(pid)
	return
}

// burnCPU spins for at least d of wall time, consuming CPU.
func burnCPU(d time.Duration) {
	start := time.Now()
	x := 0
	for time.Since(start) < d {
		for i := 0; i < 100000; i++ {
			x += i
		}
	}
	_ = x
}

// CPUPhases should charge CPU to the window it was spent in, charge ~nothing
// for sleeping, and have its windows sum to the total the process consumed.
func TestCPUPhases(t *testing.T) {
	cpu0 := perf.CPUNow()
	ph := perf.NewCPUPhases(sp.Tpid("test-cpuphases"), time.Now())

	burnCPU(60 * time.Millisecond)
	busyCPU, busyWall := ph.Mark("busy")

	time.Sleep(60 * time.Millisecond)
	idleCPU, idleWall := ph.Mark("idle")

	burnCPU(60 * time.Millisecond)
	busy2CPU, _ := ph.Mark("busy2")

	total := perf.CPUNow() - cpu0
	db.DPrintf(db.TEST, "CPUPhases busy %v (wall %v) idle %v (wall %v) busy2 %v total %v",
		busyCPU, busyWall, idleCPU, idleWall, busy2CPU, total)

	// A spinning window is charged CPU close to its wall time.
	assert.True(t, busyCPU > 40*time.Millisecond, "busy window undercharged: %v", busyCPU)
	assert.True(t, busy2CPU > 40*time.Millisecond, "busy2 window undercharged: %v", busy2CPU)
	// A sleeping window is charged almost nothing, even though wall time passed.
	assert.True(t, idleWall > 50*time.Millisecond, "idle window too short: %v", idleWall)
	assert.True(t, idleCPU < 20*time.Millisecond, "idle window overcharged: %v", idleCPU)
	// The windows partition the process's CPU: they sum to the total, without
	// double-counting (which is the point of marking rather than nesting).
	sum := busyCPU + idleCPU + busy2CPU
	assert.True(t, sum <= total+5*time.Millisecond, "windows sum %v exceeds total %v", sum, total)
	assert.True(t, sum > total-20*time.Millisecond, "windows sum %v far below total %v", sum, total)

	// Nil is a no-op, so callers that never set up phases need no guards.
	var none *perf.CPUPhases
	c, w := none.Mark("nil")
	assert.Equal(t, time.Duration(0), c)
	assert.Equal(t, time.Duration(0), w)
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
	var err error

	utime0, stime0, utime1, stime1, err = tick(pid, &t0)
	assert.Nil(t, err)
	util = perf.UtilFromCPUTimeSample(utime0, stime0, utime1, stime1, time.Since(t0).Seconds())

	db.DPrintf(db.TEST, "Util (sleep): %v", util)

	assert.True(t, util >= 0.0, "Util negative: %v", util)
	assert.True(t, util < 5.0, "Util too high: %v", util)

	const N = 3
	for i := 0; i < N; i++ {
		// Start a spinning thread to consume a core.
		go spin(done)

		// Wait for the spinning thread to start
		time.Sleep(100 * time.Millisecond)

		utime0, stime0, utime1, stime1, err = tick(pid, &t0)
		assert.Nil(t, err)
		util = perf.UtilFromCPUTimeSample(utime0, stime0, utime1, stime1, time.Since(t0).Seconds())

		db.DPrintf(db.TEST, "Util (%v spinner): %v", i, util)

		assert.True(t, util >= 100.0*float64(i+1)-10.0, "Util too low (i=%v): %v", i, util)
		assert.True(t, util < 100.0*float64(i+1)+10.0, "Util too high (i=%v): %v", i, util)
	}

	for i := 0; i < N; i++ {
		done <- true
	}
}
