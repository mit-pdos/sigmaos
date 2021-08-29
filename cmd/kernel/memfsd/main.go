package main

import (
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	"ulambda/memfsd"
	"ulambda/perf"
	"ulambda/procinit"
	"ulambda/seccomp"
)

func main() {
	linuxsched.ScanTopology()
	if os.Args[2] != "" { // initial memfsd?
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
	} else { // started as a ulambda
		name := memfsd.MEMFS + "/" + os.Args[1]
		ip, err := fsclnt.LocalIP()
		if err != nil {
			log.Fatalf("%v: no IP %v\n", os.Args[0], err)
		}
		db.Name(name)
		fsd := memfsd.MakeFsd(ip + ":0")
		fsl, err := fslibsrv.InitFs(name, fsd, nil)
		if err != nil {
			log.Fatalf("%v: InitFs failed %v\n", os.Args[0], err)
		}
		sctl := procinit.MakeProcCtl(fsl.FsLib, procinit.GetProcLayersMap())
		sctl.Started(os.Args[1])
		seccomp.LoadFilter()
		fsd.Serve()
		sctl.Exited(os.Args[1])
		fsl.ExitFs(name)
	}
}
