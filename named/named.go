package named

import (
	"os"
	"path"
	"strconv"
	"strings"

	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/fslibsrv"
	"ulambda/kernel"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/realm"
	"ulambda/repl"
	"ulambda/repldummy"
	"ulambda/replraft"
	"ulambda/sesssrv"
	// "ulambda/seccomp"
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

	var ss *sesssrv.SessSrv
	var err *np.Err
	// Replicate?
	if len(args) >= 4 {
		var config repl.Config = nil
		if args[3] == "dummy" {
			config = repldummy.MakeConfig()
		} else {
			id, r := strconv.Atoi(args[3])
			if r != nil {
				db.DFatalf("Couldn't convert id string: %v", err)
			}
			peers := strings.Split(args[4], ",")
			config = replraft.MakeRaftConfig(id, peers)
		}
		ss, err = fslibsrv.MakeReplMemFs(addr, pname, "named", config)
	} else {
		ss, err = fslibsrv.MakeReplMemFs(addr, pname, "named", nil)
	}

	if err != nil {
		db.DFatalf("%v: err %v\n", proc.GetProgram(), err)
	}

	// seccomp.LoadFilter()

	initfs(ss)

	ss.Serve()
	ss.Done()
}

var InitDir = []string{np.TMPREL, np.BOOTREL, np.KPIDSREL, np.PROCDREL, np.UXREL, np.S3REL, np.DBREL}

func initfs(ss *sesssrv.SessSrv) error {
	r := ss.Root()
	for _, n := range InitDir {
		_, err := r.Create(ctx.MkCtx("", 0, nil), n, 0777|np.DMDIR, np.OREAD)
		if err != nil {
			return err
		}
	}
	return nil
}
