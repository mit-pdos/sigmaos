package perf

import (
	"log"
	//	"os"
	//	"runtime"
	//	"runtime/pprof"
	"testing"
)

func Test10MMemfs(t *testing.T) {
	args := []string{"10000000", "memfs"}
	bw, err := MakeBandwidthTest(args)
	if err != nil {
		log.Printf("Failed")
	}
	for i := 0; i < 1000; i++ {
		bw.Work()
	}
}
