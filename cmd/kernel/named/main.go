package main

import (
	"os"

	db "ulambda/debug"
	"ulambda/linuxsched"
	"ulambda/memfsd"
	"ulambda/perf"
	"ulambda/seccomp"
)

func main() {
	linuxsched.ScanTopology()
	// If we're benchmarking, make a flame graph
	p := perf.MakePerf()
	if len(os.Args) >= 4 {
		pprofPath := os.Args[3]
		p.SetupPprof(pprofPath)
	}
	if len(os.Args) >= 5 {
		utilPath := os.Args[4]
		p.SetupCPUUtil(perf.CPU_UTIL_HZ, utilPath)
	}
	if p.RunningBenchmark() {
		// XXX For my current benchmarking setup, ZK gets core 0 all to itself.
		m := linuxsched.CreateCPUMaskOfOne(0)
		m.Set(1)
		linuxsched.SchedSetAffinityAllTasks(os.Getpid(), m)
	}
	defer p.Teardown()
	db.Name("memfsd")
	fsd := memfsd.MakeFsd(os.Args[2])
	seccomp.LoadFilter()
	fsd.Serve()
}
