package main

import (
	"log"
	"os"
	"path"

	db "ulambda/debug"
	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/memfsd"
	"ulambda/perf"
	"ulambda/realm"
	"ulambda/seccomp"
)

func main() {
	linuxsched.ScanTopology()
	// If we're benchmarking, make a flame graph
	p := perf.MakePerf()
	if len(os.Args) >= 5 {
		pprofPath := os.Args[4]
		p.SetupPprof(pprofPath)
	}
	if len(os.Args) >= 6 {
		utilPath := os.Args[5]
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
	addr := os.Args[2]
	fsd := memfsd.MakeFsd(addr)
	// Register a realm's named in the global namespace
	if len(os.Args) >= 4 {
		realmId := os.Args[3]
		path := path.Join(realm.REALM_NAMEDS, realmId, addr)
		_, err := fslibsrv.InitFs(path, fsd, nil)
		if err != nil {
			log.Fatalf("%v: InitFs failed %v\n", os.Args[0], err)
		}
	}
	seccomp.LoadFilter()
	fsd.Serve()
}
