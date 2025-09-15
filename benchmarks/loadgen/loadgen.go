package loadgen

import (
	"math/rand"
	"sync"
	"time"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
)

const (
	RAND_SEED = 12345
)

// Return true if external duration should be ignored (and favored by internal
// duration).
type Req func(*rand.Rand) (time.Duration, bool)

type LoadGenerator struct {
	sync.Mutex
	totaldur  time.Duration       // Duration of load generation.
	maxrps    int64               // Max number of requests per second.
	nthread   int                 // Number of "initiator threads", which start asynchronous requests.
	rpss      []int64             // RPS for each initiator thread.
	sleepdurs []time.Duration     // Interval at which to fire off new requests for each thread.
	req       Req                 // Request func.
	avgReqLat time.Duration       // Average request duration, when not under contention.
	res       *benchmarks.Results // Latency results
	wg        sync.WaitGroup      // Wait for request threads.
	initC     chan int64          // Channel on which initiator threads report how many total requests they initiated
}

func NewLoadGenerator(dur time.Duration, maxrps int, req Req) *LoadGenerator {
	lg := &LoadGenerator{}
	lg.totaldur = dur
	lg.maxrps = int64(maxrps)
	lg.req = req
	lg.res = nil
	lg.initC = make(chan int64)
	lg.setupInitThreads()
	return lg
}

// Set up "initiator threads", which kick off asynch request threads at a fixed
// rate. This method helps determine the fixed rate at which each of these
// initiator threads should operate.  One initiator thread can kick off roughly
// 1K asynch requests per second.
func (lg *LoadGenerator) setupInitThreads() {
	lg.rpss = make([]int64, 0)
	lg.sleepdurs = make([]time.Duration, 0)
	// Rate at which a single initiator thread can kick off asynch requests.
	N := int64(500)
	rps := lg.maxrps
	for rps > 0 {
		// Number of reqests to be fired off by this initiator thread, each second.
		r := N
		if rps < N {
			// If there are less than N request/sec left, set r = rps.
			r = rps
		}
		lg.rpss = append(lg.rpss, r)
		// Sleep duration between asynch request invocations.
		lg.sleepdurs = append(lg.sleepdurs, time.Second/time.Duration(r))
		if rps < N {
			break
		}
		rps -= N
	}
}

func (lg *LoadGenerator) runReq(i int, r *rand.Rand, store bool) time.Duration {
	defer lg.wg.Done()
	start := time.Now()
	var dur time.Duration
	var useInternalDur bool
	dur, useInternalDur = lg.req(r)
	// If not using internal (custom) duration, time external request duration
	if !useInternalDur {
		dur = time.Since(start)
	}
	if store {
		lg.res.Set(i, dur, 1.0)
	}
	return dur
}

// Find the base latency on which to base future measurements.
func (lg *LoadGenerator) Calibrate() {
	db.DPrintf(db.TEST, "Calibrating load generator")
	const N = 1000
	//	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r := rand.New(rand.NewSource(RAND_SEED))
	totalDur := time.Duration(0)
	for i := 0; i < N; i++ {
		lg.wg.Add(1)
		totalDur += lg.runReq(i, r, false)
	}
	lg.avgReqLat = totalDur / N
	// Preallocate entries. Multiply by 2 to leave a slight buffer.
	lg.res = benchmarks.NewResults(2*int(lg.maxrps*int64(lg.totaldur.Seconds()))+N, benchmarks.REQ)
	db.DPrintf(db.TEST, "Done calibrating load generator, avg latency: %v", lg.avgReqLat)
}

func (lg *LoadGenerator) warmup() {
	// TODO: some warmup scheme?
}

func (lg *LoadGenerator) initiatorThread(tid int) {
	t := time.NewTicker(lg.sleepdurs[tid])
	var nreq int64
	//	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r := rand.New(rand.NewSource(RAND_SEED + int64(tid+1)))
	start := time.Now()
	for time.Since(start) < lg.totaldur {
		<-t.C
		// Increment the number of requests done by this initiator thread.
		nreq++
		// Make space for requester to store request latency stats.
		lg.Lock()
		idx := lg.res.Append(0, 0)
		lg.Unlock()
		// Run request in a separate thread.
		lg.wg.Add(1)
		go lg.runReq(idx, r, true)
	}
	lg.initC <- nreq
}

func (lg *LoadGenerator) StatsDataString() string {
	return lg.res.String()
}

func (lg *LoadGenerator) Stats() {
	// Print raw latencies.
	db.DPrintf(db.LOADGEN, "Load generator latencies:\n%v", lg.res)
	lsum, tsum := lg.res.Summary()
	db.DPrintf(db.THROUGHPUT, "\n\nmaxrps:%v%v\n", lg.maxrps, tsum)
	db.DPrintf(db.ALWAYS, "maxrps:%v%v\n", lg.maxrps, lsum)
}

func (lg *LoadGenerator) Run() {
	db.DPrintf(db.TEST, "Start load generator")
	lg.warmup()
	// Start initiator threads.
	start := time.Now()
	for tid := 0; tid < len(lg.rpss); tid++ {
		go lg.initiatorThread(tid)
	}
	// Total requests
	var nreq int64
	for tid := 0; tid < len(lg.rpss); tid++ {
		nreq += <-lg.initC
	}
	db.DPrintf(db.ALWAYS, "Avg req/sec client-side: %v", float64(nreq)/time.Since(start).Seconds())
	lg.wg.Wait()
	db.DPrintf(db.ALWAYS, "Avg req/sec server-side: %v", float64(nreq)/time.Since(start).Seconds())
}
