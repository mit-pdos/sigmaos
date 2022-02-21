package named

import (
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"ulambda/ctx"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/kernel"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/realm"
	"ulambda/replraft"
	"ulambda/seccomp"
)

func Run(args []string) {
	linuxsched.ScanTopology()
	// If we're benchmarking, make a flame graph
	p := perf.MakePerf()
	if len(args) >= 6 {
		pprofPath := args[5]
		p.SetupPprof(pprofPath)
	}
	if len(args) >= 7 {
		utilPath := args[6]
		p.SetupCPUUtil(perf.CPU_UTIL_HZ, utilPath)
	}
	if p.RunningBenchmark() {
		// XXX For my current benchmarking setup, ZK gets core 0 all to itself.
		m := linuxsched.CreateCPUMaskOfOne(0)
		m.Set(1)
		linuxsched.SchedSetAffinityAllTasks(os.Getpid(), m)
	}
	defer p.Teardown()

	addr := args[1]

	// A realm's named in the global namespace
	realmId := args[2]
	var pname string
	if realmId != kernel.NO_REALM {
		pname = path.Join(realm.REALM_NAMEDS, realmId)
	}

	var fss *fssrv.FsServer
	var err *np.Err
	// Replicate?
	if len(args) >= 4 {
		id, r := strconv.Atoi(args[3])
		if r != nil {
			log.Fatalf("Couldn't convert id string: %v", err)
		}
		peers := strings.Split(args[4], ",")
		config := replraft.MakeRaftConfig(id, peers)
		fss, _, _, err = fslibsrv.MakeReplMemFs(addr, pname, "named", config)
	} else {
		fss, _, _, err = fslibsrv.MakeReplMemFs(addr, pname, "named", nil)
	}

	if err != nil {
		log.Fatalf("FATAL %v: err %v\n", proc.GetProgram(), err)
	}

	seccomp.LoadFilter()

	initfs(fss)

	fss.Serve()
	fss.Done()
}

var InitDir = []string{np.TMPREL, np.BOOTREL, np.KPIDSREL, np.PROCDREL}

func initfs(fss *fssrv.FsServer) error {
	r := fss.Root()
	for _, n := range InitDir {
		_, err := r.Create(ctx.MkCtx("", 0, nil), n, 0777|np.DMDIR, np.OREAD)
		if err != nil {
			return err
		}
	}
	return nil
}
