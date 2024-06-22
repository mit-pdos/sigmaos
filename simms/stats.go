package simms

import (
	"github.com/montanaflynn/stats"

	db "sigmaos/debug"
)

type Stats struct {
	lat []uint64
}

func NewStats() *Stats {
	return &Stats{
		lat: []uint64{},
	}
}

func (st *Stats) Update(reps []*Reply) {
	for _, rep := range reps {
		st.lat = append(st.lat, rep.lat)
	}
}

func (st *Stats) TotalRequests() int {
	return len(st.lat)
}

func (st *Stats) AvgLatency() float64 {
	flat := make([]float64, len(st.lat))
	for i := range st.lat {
		flat[i] = float64(st.lat[i])
	}
	l, err := stats.Mean(flat)
	if err != nil {
		db.DFatalf("Error Mean in Results.Mean: %v", err)
	}
	return l
}
