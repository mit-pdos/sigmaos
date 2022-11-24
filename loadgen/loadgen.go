package loadgen

import (
	"sync"
	"time"

	db "sigmaos/debug"
)

type Req func()

type LoadGenerator struct {
	totaldur  time.Duration   // Duration of load generation.
	sleepdur  time.Duration   // Interval at which to fire off new requests.
	maxrps    int64           // Max number of requests per second.
	req       Req             // Request func.
	avgReqLat time.Duration   // Average request duration, when not under contention.
	lats      []time.Duration // Latencies.
	wg        sync.WaitGroup  // Wait for request threads.
}

func MakeLoadGenerator(dur time.Duration, maxrps int, req Req) *LoadGenerator {
	lg := &LoadGenerator{}
	lg.totaldur = dur
	lg.sleepdur = time.Second / time.Duration(maxrps)
	lg.maxrps = int64(maxrps)
	lg.req = req
	lg.lats = nil
	return lg
}

func (lg *LoadGenerator) runReq(i int, store bool) {
	defer lg.wg.Done()
	start := time.Now()
	lg.req()
	if store {
		lg.lats[i] = time.Since(start)
	}
}

// Find the base latency on which to base future measurements.
func (lg *LoadGenerator) calibrate() {
	const N = 1000
	start := time.Now()
	for i := 0; i < N; i++ {
		lg.wg.Add(1)
		lg.runReq(i, false)
	}
	lg.avgReqLat = time.Since(start) / N
	// Preallocate entries.
	lg.lats = make([]time.Duration, 0, lg.maxrps*int64(lg.totaldur/lg.avgReqLat))
}

func (lg *LoadGenerator) warmup() {
	// TODO: some warmup scheme?
}

func (lg *LoadGenerator) Stats() {
	var tot time.Duration
	for _, l := range lg.lats {
		tot += l
	}
	db.DPrintf(db.ALWAYS, "Average latency: %v", tot/time.Duration(len(lg.lats)))
}

func (lg *LoadGenerator) Run() {
	// Calibrate.
	lg.calibrate()
	lg.warmup()
	t := time.NewTicker(lg.sleepdur)
	var i int
	start := time.Now()
	for ; time.Since(start) < lg.totaldur; i++ {
		<-t.C
		// Make space for thread to store request latency.
		lg.lats = append(lg.lats, lg.totaldur)
		// Run request in a separate thread.
		lg.wg.Add(1)
		go lg.runReq(i, true)
	}
	db.DPrintf(db.ALWAYS, "Avg req/sec: %v", float64(i)/time.Since(start).Seconds())
	lg.wg.Wait()
	lg.Stats()
}
