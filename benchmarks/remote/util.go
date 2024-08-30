package remote

import (
	"strconv"
	"time"
)

// Construct comma-separated string of durations
func dursToString(durs []time.Duration) string {
	durStr := ""
	for i, d := range durs {
		durStr += d.String()
		if i < len(durs)-1 {
			durStr += ","
		}
	}
	return durStr
}

// Construct comma-separated string of RPS
func rpsToString(rps []int) string {
	rpsStr := ""
	for i, r := range rps {
		rpsStr += strconv.Itoa(r)
		if i < len(rps)-1 {
			rpsStr += ","
		}
	}
	return rpsStr
}
