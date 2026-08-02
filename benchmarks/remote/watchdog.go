package remote

import (
	"sync"
	"time"

	db "sigmaos/debug"
)

// How long a single benchmark may run before the watchdog starts complaining.
// A big job plus a cluster boot legitimately takes a few minutes, but past
// this point silence usually means something is wedged (a hung proc, a cluster
// that never came up, a driver waiting on a reply that will never arrive)
// rather than slow progress, and the run is otherwise indistinguishable from
// one which is simply taking a while.
const benchWatchdogInterval = 5 * time.Minute

// Warns periodically while a benchmark is still running, naming the phase it
// is stuck in so the warning points somewhere.
type watchdog struct {
	mu         sync.Mutex
	name       string
	phase      string
	phaseStart time.Time
	start      time.Time
	done       chan struct{}
	stopOnce   sync.Once
}

func startWatchdog(name string) *watchdog {
	now := time.Now()
	w := &watchdog{
		name:       name,
		phase:      "starting",
		phaseStart: now,
		start:      now,
		done:       make(chan struct{}),
	}
	go w.run()
	return w
}

func (w *watchdog) run() {
	t := time.NewTicker(benchWatchdogInterval)
	defer t.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-t.C:
			w.mu.Lock()
			phase, phaseFor := w.phase, time.Since(w.phaseStart)
			w.mu.Unlock()
			db.DPrintf(db.ALWAYS, "WARNING: benchmark %v has been running for %v, currently in phase %q for %v; it may be stuck",
				w.name, time.Since(w.start).Truncate(time.Second), phase, phaseFor.Truncate(time.Second))
		}
	}
}

// Note which stage of the benchmark is now running, and reset the per-phase
// timer. Safe to call after Stop.
func (w *watchdog) SetPhase(phase string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.phase = phase
	w.phaseStart = time.Now()
}

func (w *watchdog) Stop() {
	w.stopOnce.Do(func() { close(w.done) })
}
