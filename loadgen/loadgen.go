package loadgen

import (
	"sync"
	"time"

	"github.com/montanaflynn/stats"

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
	data := make([]float64, len(lg.lats))
	for i, l := range lg.lats {
		data[i] = float64(l.Microseconds()) / 1000.0
		db.DPrintf("TEST", "Latency %v", l)
	}
	mean, err := stats.Mean(data)
	if err != nil {
		db.DFatalf("Error calculating mean: %v", err)
	}
	median, err := stats.Percentile(data, 50)
	if err != nil {
		db.DFatalf("Error calculating percentile 50: %v", err)
	}
	p75, err := stats.Percentile(data, 75)
	if err != nil {
		db.DFatalf("Error calculating percentile 75: %v", err)
	}
	p90, err := stats.Percentile(data, 90)
	if err != nil {
		db.DFatalf("Error calculating percentile 90: %v", err)
	}
	p99, err := stats.Percentile(data, 99)
	if err != nil {
		db.DFatalf("Error calculating percentile 99: %v", err)
	}
	p999, err := stats.Percentile(data, 99.9)
	if err != nil {
		db.DFatalf("Error calculating percentile 99.9: %v", err)
	}
	p9999, err := stats.Percentile(data, 99.99)
	if err != nil {
		db.DFatalf("Error calculating percentile 99.99: %v", err)
	}
	p100, err := stats.Percentile(data, 100)
	if err != nil {
		db.DFatalf("Error calculating percentile 100.0: %v", err)
	}
	db.DPrintf(db.ALWAYS,
		"\nLatency Stats:\n\nMean: %vms\n50%%: %vms\n75%%: %vms\n90%%: %vms\n99%%: %vms\n99.9%%: %vms\n99.99%%: %vms\n100%%: %vms",
		mean, median, p75, p90, p99, p999, p9999, p100)
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
