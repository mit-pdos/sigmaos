package simms

import (
	"github.com/montanaflynn/stats"

	db "sigmaos/debug"
)

type WorkloadStats struct {
	lat [][]uint64
}

func NewWorkloadStats() *WorkloadStats {
	return &WorkloadStats{
		lat: [][]uint64{},
	}
}

func (st *WorkloadStats) Tick(reps []*Reply) {
	lats := make([]uint64, 0, len(reps))
	for _, rep := range reps {
		lats = append(lats, rep.GetLatency())
	}
	st.lat = append(st.lat, lats)
}

func (st *WorkloadStats) TotalRequests() int {
	tot := 0
	for _, l := range st.lat {
		tot += len(l)
	}
	return tot
}

func (st *WorkloadStats) AvgLatency() float64 {
	flat := make([]float64, st.TotalRequests())
	n := 0
	for i := range st.lat {
		for j := range st.lat[i] {
			flat[n] = float64(st.lat[i][j])
			n++
		}
	}
	l, err := stats.Mean(flat)
	if err != nil {
		db.DFatalf("Error Mean in Results.Mean: %v", err)
	}
	return l
}

func (st *WorkloadStats) GetLatencies() [][]uint64 {
	return st.lat
}

type ServiceInstanceStats struct {
	t                *uint64
	time             []uint64
	requestsInFlight []uint64
	util             []float64
	latency          [][]uint64
}

func NewServiceInstanceStats(t *uint64) *ServiceInstanceStats {
	return &ServiceInstanceStats{
		t:                t,
		time:             []uint64{},
		requestsInFlight: []uint64{},
		util:             []float64{},
		latency:          [][]uint64{},
	}
}

func (sis *ServiceInstanceStats) Tick(processing []*Request, nslots int, replies []*Reply) {
	sis.time = append(sis.time, *sis.t)
	sis.requestsInFlight = append(sis.requestsInFlight, uint64(len(processing)))
	sis.util = append(sis.util, float64(len(processing))/float64(nslots))
	lats := make([]uint64, 0, len(replies))
	for _, r := range replies {
		lats = append(lats, r.GetLatency())
	}
	sis.latency = append(sis.latency, lats)
}
