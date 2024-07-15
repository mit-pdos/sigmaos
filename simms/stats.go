package simms

import (
	"fmt"
	"math"

	"github.com/montanaflynn/stats"

	db "sigmaos/debug"
)

type ServiceStats struct {
	lat    [][]uint64
	rstats *RecordedStats
}

func NewServiceStats() *ServiceStats {
	return &ServiceStats{
		lat:    [][]uint64{},
		rstats: NewRecordedStats(0),
	}
}

func (st *ServiceStats) Tick(t uint64, reps []*Reply) {
	lats := make([]uint64, 0, len(reps))
	for _, rep := range reps {
		lats = append(lats, rep.GetLatency())
	}
	st.lat = append(st.lat, lats)
	st.rstats.record(t, st)
}

func (st *ServiceStats) RecordStats(window int) {
	st.rstats.window = window
}

func (st *ServiceStats) StopRecordingStats() {
	st.rstats.window = 0
}

func totalNReqs(lat [][]uint64) int {
	tot := 0
	for _, l := range lat {
		tot += len(l)
	}
	return tot
}

func (st *ServiceStats) TotalRequests() int {
	return totalNReqs(st.lat)
}

// Convert a 2-D slice of latencies, into a 2-D slice of float latencies
func latenciesAsFloatSlice(lat [][]uint64) [][]float64 {
	floats := make([][]float64, len(lat))
	for i := range lat {
		floats[i] = make([]float64, len(lat[i]))
		for j := range lat[i] {
			floats[i][j] = float64(lat[i][j])
		}
	}
	return floats
}

// Given a 2-D slice of latencies, in which the first dimension is time
// (ticks), and the second dimension is latency for requests completed in a
// given tick, flatten into a 1-D slice of float latencies
func latenciesAsFlatFloatSlice(lat [][]uint64) []float64 {
	nreq := totalNReqs(lat)
	flat := make([]float64, nreq)
	n := 0
	for i := range lat {
		for j := range lat[i] {
			flat[n] = float64(lat[i][j])
			n++
		}
	}
	return flat
}

func avgLatency(lat [][]uint64) float64 {
	if totalNReqs(lat) == 0 {
		return 0.0
	}
	flat := latenciesAsFlatFloatSlice(lat)
	l, err := stats.Mean(flat)
	if err != nil {
		db.DFatalf("Error calculating mean: %v", err)
	}
	return l
}

// Average latency over requests completed in the last N ticks
func (st *ServiceStats) AvgLatencyLastNTicks(nticks int) float64 {
	idx := 0
	if len(st.lat) > nticks {
		idx = len(st.lat) - nticks
	}
	// Only consider last N ticks
	lats := st.lat[idx:]
	return avgLatency(lats)
}

// Average latency over all requests
func (st *ServiceStats) AvgLatency() float64 {
	return avgLatency(st.lat)
}

func percentileLatency(p float64, lat [][]uint64) float64 {
	if totalNReqs(lat) == 0 {
		return 0.0
	}
	flat := latenciesAsFlatFloatSlice(lat)
	percentile, err := stats.Percentile(flat, p)
	if err != nil {
		db.DFatalf("Error calculating percentile %v: %v", p, err)
	}
	return percentile
}

// Average latency over requests completed in the last N ticks
func (st *ServiceStats) PercentileLatencyLastNTicks(p float64, nticks int) float64 {
	idx := 0
	if len(st.lat) > nticks {
		idx = len(st.lat) - nticks
	}
	// Only consider last N ticks
	lats := st.lat[idx:]
	return percentileLatency(p, lats)
}

// Percentile over all requests
func (st *ServiceStats) PercentileLatency(p float64) float64 {
	return percentileLatency(p, st.lat)
}

func (st *ServiceStats) GetLatencies() [][]uint64 {
	return st.lat
}

func (st *ServiceStats) GetRecordedStats() *RecordedStats {
	return st.rstats
}

type ServiceInstanceStats struct {
	t                *uint64
	time             []uint64
	Ready            []bool
	requestsInFlight []uint64
	Util             []float64
	latency          [][]uint64
}

// Create new service instance stat, with zeroed stats for previous time steps
func NewServiceInstanceStats(t *uint64) *ServiceInstanceStats {
	return &ServiceInstanceStats{
		t:                t,
		time:             make([]uint64, *t+1),
		Ready:            make([]bool, *t+1),
		requestsInFlight: make([]uint64, *t+1),
		Util:             make([]float64, *t+1),
		latency:          make([][]uint64, *t+1),
	}
}

func (sis *ServiceInstanceStats) Tick(ready bool, processing []*Request, nslots int, replies []*Reply) {
	sis.time = append(sis.time, *sis.t)
	sis.Ready = append(sis.Ready, ready)
	sis.requestsInFlight = append(sis.requestsInFlight, uint64(len(processing)))
	sis.Util = append(sis.Util, float64(len(processing))/float64(nslots))
	lats := make([]uint64, 0, len(replies))
	for _, r := range replies {
		lats = append(lats, r.GetLatency())
	}
	sis.latency = append(sis.latency, lats)
}

type RecordedStats struct {
	window     int
	Time       []uint64
	AvgLatency []float64
	P50Latency []float64
	P90Latency []float64
	P99Latency []float64
}

func NewRecordedStats(window int) *RecordedStats {
	return &RecordedStats{
		window:     window,
		Time:       []uint64{},
		AvgLatency: []float64{},
		P50Latency: []float64{},
		P90Latency: []float64{},
		P99Latency: []float64{},
	}
}

func roundToHundredth(f float64) float64 {
	return math.Round(f*100.0) / 100.0
}

// Optionally record workload stats in a sliding window
func (rst *RecordedStats) record(t uint64, st *ServiceStats) {
	if rst.window > 0 {
		rst.Time = append(rst.Time, t)
		rst.AvgLatency = append(rst.AvgLatency, roundToHundredth(st.AvgLatencyLastNTicks(rst.window)))
		rst.P50Latency = append(rst.P50Latency, roundToHundredth(st.PercentileLatencyLastNTicks(50.0, rst.window)))
		rst.P90Latency = append(rst.P90Latency, roundToHundredth(st.PercentileLatencyLastNTicks(90.0, rst.window)))
		rst.P99Latency = append(rst.P99Latency, roundToHundredth(st.PercentileLatencyLastNTicks(99.0, rst.window)))
	}
}

func (rst *RecordedStats) String() string {
	return fmt.Sprintf("&{ window:%v\n\tavg:%v\n\tp50:%v\n\tp90:%v\n\tp99:%v\n}", rst.window, rst.AvgLatency, rst.P50Latency, rst.P90Latency, rst.P99Latency)
}
