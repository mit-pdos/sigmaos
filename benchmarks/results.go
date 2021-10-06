package benchmarks

import (
	"fmt"
	"log"

	"github.com/montanaflynn/stats"
)

type Result struct {
	Throughput float64 // ops/usec
	Latency    float64 // usecs
}

func MakeResult() *Result {
	r := &Result{}
	r.Throughput = 0.0
	r.Latency = 0.0
	return r
}

func (r *Result) set(throughput, latency float64) {
	r.Throughput = throughput
	r.Latency = latency
}

type RawResults struct {
	Data []*Result
}

func MakeRawResults(nTrials int) *RawResults {
	r := &RawResults{}
	r.Data = make([]*Result, nTrials)
	for i := 0; i < nTrials; i++ {
		r.Data[i] = MakeResult()
	}
	return r
}

func (r *RawResults) Mean() *Result {
	tpt := make([]float64, len(r.Data))
	lat := make([]float64, len(r.Data))

	for i := range r.Data {
		tpt[i] = r.Data[i].Throughput
		lat[i] = r.Data[i].Latency
	}

	res := MakeResult()

	t, err := stats.Mean(tpt)
	if err != nil {
		log.Fatalf("Error Mean in RawResults.Mean: %v", err)
	}
	res.Throughput = t

	l, err := stats.Mean(lat)
	if err != nil {
		log.Fatalf("Error Mean in RawResults.Mean: %v", err)
	}
	res.Latency = l

	return res
}

func (r *Result) String() string {
	return fmt.Sprintf("&{ Throughput (ops/usec):%f Latency (usec):%f }", r.Throughput, r.Latency)
}
