package loadgen

import (
	"sync"
	"time"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
)

type Req func()

type LoadGenerator struct {
	totaldur  time.Duration       // Duration of load generation.
	sleepdur  time.Duration       // Interval at which to fire off new requests.
	maxrps    int64               // Max number of requests per second.
	req       Req                 // Request func.
	avgReqLat time.Duration       // Average request duration, when not under contention.
	res       *benchmarks.Results // Latency results
	wg        sync.WaitGroup      // Wait for request threads.
}

func MakeLoadGenerator(dur time.Duration, maxrps int, req Req) *LoadGenerator {
	lg := &LoadGenerator{}
	lg.totaldur = dur
	lg.sleepdur = time.Second / time.Duration(maxrps)
	lg.maxrps = int64(maxrps)
	lg.req = req
	lg.res = nil
	return lg
}

func (lg *LoadGenerator) runReq(i int, store bool) {
	defer lg.wg.Done()
	start := time.Now()
	lg.req()
	if store {
		lg.res.Set(i, time.Since(start), 1.0)
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
	lg.res = benchmarks.MakeResults(int(lg.maxrps*int64(lg.totaldur/lg.avgReqLat)), "req")
}

func (lg *LoadGenerator) warmup() {
	// TODO: some warmup scheme?
}

func (lg *LoadGenerator) Stats() {
	for _, l := range lg.res.RawLatencies() {
		db.DPrintf("LOADGEN", "Latency %v", l)
	}
	mean, _ := lg.res.Mean()
	median, _ := lg.res.Percentile(50)
	p75, _ := lg.res.Percentile(75)
	p90, _ := lg.res.Percentile(90)
	p99, _ := lg.res.Percentile(99)
	p999, _ := lg.res.Percentile(99.9)
	p9999, _ := lg.res.Percentile(99.99)
	p100, _ := lg.res.Percentile(100)
	db.DPrintf(db.ALWAYS,
		"\n= Latency Stats:\n Mean: %v\n 50%%: %v\n 75%%: %v\n 90%%: %v\n 99%%: %v\n 99.9%%: %v\n 99.99%%: %v\n 100%%: %v",
		mean, median, p75, p90, p99, p999, p9999, p100)
}

func (lg *LoadGenerator) Run() {
	db.DPrintf("TEST", "Start load generator")
	// Calibrate.
	lg.calibrate()
	db.DPrintf("TEST", "Done calibrating load generator, avg latency: %v", lg.avgReqLat)
	lg.warmup()
	t := time.NewTicker(lg.sleepdur)
	var i int
	start := time.Now()
	for ; time.Since(start) < lg.totaldur; i++ {
		<-t.C
		// Make space for thread to store request latency.
		lg.res.Append(0, 0)
		// Run request in a separate thread.
		lg.wg.Add(1)
		go lg.runReq(i, true)
	}
	db.DPrintf(db.ALWAYS, "Avg req/sec: %v", float64(i)/time.Since(start).Seconds())
	lg.wg.Wait()
	lg.Stats()
}
