package remote

import (
	"testing"
	"time"
)

func TestWatchdogFires(t *testing.T) {
	w := startWatchdog("fake-bench")
	w.SetPhase("run benchmark clients")
	time.Sleep(50 * time.Millisecond)
	w.Stop()
	w.Stop() // idempotent
	w.SetPhase("after stop")
}
