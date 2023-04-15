package main

import (
	"fmt"
	"testing"
	"time"

	sp "sigmaos/sigmap"
)

func TestLoop(t *testing.T) {
	dur := time.Duration(time.Second * 5)
	mem := make([]byte, 2*sp.GBYTE)
	iter := uint64(0)
	start := time.Now()
	for time.Since(start) < dur {
		iter += rw(mem)
	}
	tput := float64(iter) / time.Since(start).Seconds()
	fmt.Printf("time %v iter %v tput %.2f iter/s\n", time.Since(start), iter, tput)
}
