package benchmarks

import (
	"fmt"
	"time"

	"github.com/montanaflynn/stats"

	db "sigmaos/debug"
)

type Results struct {
	dur  []time.Duration // Duration.
	amt  []float64       // Amount (e.g. bytes read/written).
	unit string
	lat  []float64 // To avoid converting to float slices many times for the stats library.
	tpt  []float64 // To avoid converting to float slices many times for the stats library.
}

func MakeResults(n int, unit string) *Results {
	r := &Results{}
	r.dur = make([]time.Duration, 0, n)
	r.amt = make([]float64, 0, n)
	r.unit = unit
	r.lat = nil
	r.tpt = nil
	return r
}

// Add a data point, and return the
func (r *Results) Append(d time.Duration, amt float64) int {
	i := len(r.dur)
	r.dur = append(r.dur, d)
	r.amt = append(r.amt, amt)
	// Kill cache
	r.lat = nil
	r.tpt = nil
	return i
}

// Set the ith data point.
func (r *Results) Set(i int, d time.Duration, amt float64) {
	r.dur[i] = d
	r.amt[i] = amt
	// Kill cache
	r.lat = nil
	r.tpt = nil
}

func (r *Results) Mean() (time.Duration, float64) {
	var avglat time.Duration
	var avgtpt float64

	lat, tpt := r.toFloats()

	l, err := stats.Mean(lat)
	if err != nil {
		db.DFatalf("Error Mean in Results.Mean: %v", err)
	}
	avglat = time.Duration(int64(l))

	t, err := stats.Mean(tpt)
	if err != nil {
		db.DFatalf("Error Mean in Results.Mean: %v", err)
	}
	avgtpt = t

	return avglat, avgtpt
}

func (r *Results) StdDev() (time.Duration, float64) {
	var stdlat time.Duration
	var stdtpt float64

	lat, tpt := r.toFloats()

	l, err := stats.StandardDeviation(lat)
	if err != nil {
		db.DFatalf("Error StandardDeviation in Results.StandardDeviation: %v", err)
	}
	stdlat = time.Duration(int64(l))

	t, err := stats.StandardDeviation(tpt)
	if err != nil {
		db.DFatalf("Error StandardDeviation in Results.StandardDeviation: %v", err)
	}
	stdtpt = t

	return stdlat, stdtpt
}

// Calculate percentile. Note, this calculates the percentile separately for
// tpt & latency, and thus the results for each may correspond to different
// points. (i.e., the lowest-latency datapoint may not be the lowest-throughput
// datapoint).
func (r *Results) Percentile(p float64) (time.Duration, float64) {
	var plat time.Duration
	var ptpt float64

	if p < 0.0 || p > 100.0 {
		db.DFatalf("Bad percentile, not in [0, 100.0]: %v", p)
	}

	lat, tpt := r.toFloats()

	l, err := stats.Percentile(lat, p)
	if err != nil {
		db.DFatalf("Error calculating percentile %v: %v", p, err)
	}
	plat = time.Duration(int64(l))

	t, err := stats.Percentile(tpt, p)
	if err != nil {
		db.DFatalf("Error calculating percentile %v: %v", p, err)
	}
	ptpt = t

	return plat, ptpt
}

// Convert time.Duration to float for stats library, and calculate tpt. Cache
// the results of conversion.
func (r *Results) toFloats() ([]float64, []float64) {
	// If already calculated & cached, return cached conversion.
	if r.lat != nil && r.tpt != nil {
		return r.lat, r.tpt
	}

	lat := make([]float64, len(r.dur))
	tpt := make([]float64, len(r.amt))

	for i := range r.dur {
		lat[i] = float64(r.dur[i])
		tpt[i] = r.amt[i] / r.dur[i].Seconds()
	}

	// Cache conversion.
	r.lat = lat
	r.tpt = tpt

	return lat, tpt
}

// Print summary of results.
func (r *Results) Summary() (string, string) {
	meanL, meanT := r.Mean()
	stdL, stdT := r.StdDev()
	medianL, medianT := r.Percentile(50)
	p75L, p75T := r.Percentile(75)
	p90L, p90T := r.Percentile(90)
	p99L, p99T := r.Percentile(99)
	p999L, p999T := r.Percentile(99.9)
	p9999L, p9999T := r.Percentile(99.99)
	p100L, p100T := r.Percentile(100)
	fstring := "Stats:\n Mean: %v\n Std: %v\n 50: %v\n 75: %v\n 90: %v\n 99: %v\n 99.9: %v\n 99.99: %v\n 100: %v"
	lsum := fmt.Sprintf("\n= Latency "+fstring,
		meanL, stdL, medianL, p75L, p90L, p99L, p999L, p9999L, p100L)
	//	lsum := fmt.Sprintf("\n= Latency "+fstring,
	//		meanL, stdL, medianL, p75L, p90L, p99L, p999L, p9999L, p100L)
	tsum := fmt.Sprintf("\n= Throughput "+fstring,
		meanT, stdT, medianT, p75T, p90T, p99T, p999T, p9999T, p100T)
	return lsum, tsum
}

func (r *Results) String() string {
	if len(r.dur) == 0 {
		db.DFatalf("Error no results")
	}
	s := ""
	for i := 0; i < len(r.dur); i++ {
		s += fmt.Sprintf("&{ Lat %v Tpt %f %v/sec }\n", r.dur[i], r.amt[i]/r.dur[i].Seconds(), r.unit)
	}
	return s
}
