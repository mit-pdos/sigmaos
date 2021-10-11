package main

import (
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/perf"
	"ulambda/realm"
	"ulambda/replraft"
	"ulambda/seccomp"
	"ulambda/sync"
)

func main() {
	// Usage: <named> pid address realmId <peerId> <peers> <pprofPath> <utilPath>

	linuxsched.ScanTopology()
	// If we're benchmarking, make a flame graph
	p := perf.MakePerf()
	if len(os.Args) >= 7 {
		pprofPath := os.Args[6]
		p.SetupPprof(pprofPath)
	}
	if len(os.Args) >= 8 {
		utilPath := os.Args[7]
		p.SetupCPUUtil(perf.CPU_UTIL_HZ, utilPath)
	}
	if p.RunningBenchmark() {
		// XXX For my current benchmarking setup, ZK gets core 0 all to itself.
		m := linuxsched.CreateCPUMaskOfOne(0)
		m.Set(1)
		linuxsched.SchedSetAffinityAllTasks(os.Getpid(), m)
	}
	defer p.Teardown()

	addr := os.Args[2]

	// A realm's named in the global namespace
	realmId := os.Args[3]
	var pname string
	if realmId != realm.NO_REALM {
		pname = path.Join(realm.REALM_NAMEDS, realmId, addr)
	}

	var fss *fssrv.FsServer
	// Replicate?
	if len(os.Args) >= 5 {
		id, err := strconv.Atoi(os.Args[4])
		if err != nil {
			log.Fatalf("Couldn't convert id string: %v", err)
		}
		peers := strings.Split(os.Args[5], ",")
		config := replraft.MakeRaftConfig(id, peers)
		fss, _, _ = fslibsrv.MakeReplMemfs(addr, pname, "named", config)
	} else {
		fss, _, _ = fslibsrv.MakeReplMemfs(addr, pname, "named", nil)
	}

	seccomp.LoadFilter()

	// Mark self as started if this isn't the initial named
	isInitNamed := false
	for _, a := range fslib.Named() {
		if a == addr {
			isInitNamed = true
			break
		}
	}
	if !isInitNamed {
		namedStartCond := sync.MakeCond(fslib.MakeFsLib("named"), path.Join(named.BOOT, addr), nil)
		namedStartCond.Destroy()
	}

	fss.Serve()
}
