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
	"ulambda/kernel"
	"ulambda/linuxsched"
	"ulambda/named"
	"ulambda/perf"
	"ulambda/realm"
	"ulambda/replraft"
	"ulambda/seccomp"
	"ulambda/sync"
)

func main() {
	// Usage: <named> address realmId <peerId> <peers> <pprofPath> <utilPath>

	linuxsched.ScanTopology()
	// If we're benchmarking, make a flame graph
	p := perf.MakePerf()
	if len(os.Args) >= 6 {
		pprofPath := os.Args[5]
		p.SetupPprof(pprofPath)
	}
	if len(os.Args) >= 7 {
		utilPath := os.Args[6]
		p.SetupCPUUtil(perf.CPU_UTIL_HZ, utilPath)
	}
	if p.RunningBenchmark() {
		// XXX For my current benchmarking setup, ZK gets core 0 all to itself.
		m := linuxsched.CreateCPUMaskOfOne(0)
		m.Set(1)
		linuxsched.SchedSetAffinityAllTasks(os.Getpid(), m)
	}
	defer p.Teardown()

	addr := os.Args[1]

	// A realm's named in the global namespace
	realmId := os.Args[2]
	var pname string
	if realmId != kernel.NO_REALM {
		pname = path.Join(realm.REALM_NAMEDS, realmId)
	}

	var fss *fssrv.FsServer
	// Replicate?
	if len(os.Args) >= 4 {
		id, err := strconv.Atoi(os.Args[3])
		if err != nil {
			log.Fatalf("Couldn't convert id string: %v", err)
		}
		peers := strings.Split(os.Args[4], ",")
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
		namedStartCond := sync.MakeCond(fslib.MakeFsLib("named"), path.Join(named.BOOT, addr), nil, true)
		namedStartCond.Destroy()
	}

	fss.Serve()
}
