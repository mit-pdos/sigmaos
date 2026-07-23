package hpsearch

import "time"

var sink int = 0

func SleepBurn(d time.Duration) {
	start := time.Now()
	for time.Since(start) < d {
		sink++
	}
}
