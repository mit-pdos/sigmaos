package benchmarks

import (
	"fmt"

	"github.com/montanaflynn/stats"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Result struct {
	Throughput float64   // ops/usec
	Latency    float64   // usecs
	NRPC       np.Tseqno // Number of RPCs
}

func MakeResult() *Result {
	r := &Result{}
	r.Throughput = 0.0
	r.Latency = 0.0
	return r
}

func (r *Result) Set(throughput, latency float64, nRPC np.Tseqno) {
	r.Throughput = throughput
	r.Latency = latency
	r.NRPC = nRPC
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
	nRPC := make([]float64, len(r.Data))

	for i := range r.Data {
		tpt[i] = r.Data[i].Throughput
		lat[i] = r.Data[i].Latency
		nRPC[i] = float64(r.Data[i].NRPC)
	}

	res := MakeResult()

	t, err := stats.Mean(tpt)
	if err != nil {
		db.DFatalf("Error Mean in RawResults.Mean: %v", err)
	}
	res.Throughput = t

	l, err := stats.Mean(lat)
	if err != nil {
		db.DFatalf("Error Mean in RawResults.Mean: %v", err)
	}
	res.Latency = l

	n, err := stats.Mean(nRPC)
	if err != nil {
		db.DFatalf("Error Mean in RawResults.Mean: %v", err)
	}
	res.NRPC = np.Tseqno(n)

	return res
}

func (r *RawResults) StandardDeviation() *Result {
	tpt := make([]float64, len(r.Data))
	lat := make([]float64, len(r.Data))
	nRPC := make([]float64, len(r.Data))

	for i := range r.Data {
		tpt[i] = r.Data[i].Throughput
		lat[i] = r.Data[i].Latency
		nRPC[i] = float64(r.Data[i].NRPC)
	}

	res := MakeResult()

	t, err := stats.StandardDeviation(tpt)
	if err != nil {
		db.DFatalf("Error StandardDeviation in RawResults.StandardDeviation: %v", err)
	}
	res.Throughput = t

	l, err := stats.StandardDeviation(lat)
	if err != nil {
		db.DFatalf("Error StandardDeviation in RawResults.StandardDeviation: %v", err)
	}
	res.Latency = l

	n, err := stats.StandardDeviation(nRPC)
	if err != nil {
		db.DFatalf("Error StandardDeviation in RawResults.StandardDeviation: %v", err)
	}
	res.NRPC = np.Tseqno(n)

	return res
}

func (r *Result) String() string {
	return fmt.Sprintf("&{ Throughput (ops/usec):%f Latency (usec):%f NRPC:%d }", r.Throughput, r.Latency, r.NRPC)
}
