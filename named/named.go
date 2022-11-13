package named

import (
	"path"
	"strconv"
	"strings"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/fslibsrv"
	"sigmaos/kernel"
	np "sigmaos/ninep"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/realm"
	"sigmaos/repl"
	"sigmaos/repldummy"
	"sigmaos/replraft"
	"sigmaos/sesssrv"
	// "sigmaos/seccomp"
)

func Run(args []string) {
	perf.Hz()
	p := perf.MakePerf("NAMED")
	defer p.Done()

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
			config = replraft.MakeRaftConfig(id, peers, true)
		}
		ss, err = fslibsrv.MakeReplMemFs(addr, pname, "named", config, nil)
	} else {
		ss, err = fslibsrv.MakeReplMemFs(addr, pname, "named", nil, nil)
	}

	if err != nil {
		db.DFatalf("%v: err %v\n", proc.GetProgram(), err)
	}

	// seccomp.LoadFilter()

	initfs(ss)

	db.DPrintf("NAMED", "Named started rid:%v ip:%v", realmId, ss.MyAddr())

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
