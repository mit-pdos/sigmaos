package named

import (
	"log"
	"strconv"
	"strings"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/repldummy"
	"sigmaos/replraft"
	"sigmaos/serr"
	"sigmaos/sesssrv"
	sp "sigmaos/sigmap"
	// "sigmaos/seccomp"
)

func Run(args []string) {
	perf.Hz()
	p, r := perf.MakePerf(perf.NAMED)
	if r != nil {
		log.Printf("MakePerf err %v\n", r)
	}
	defer p.Done()

	addr := args[1]
	realmId := args[2]
	var ss *sesssrv.SessSrv
	var err *serr.Err
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
		ss, err = memfssrv.MakeReplMemFs(addr, "", "named", config)
	} else {
		ss, err = memfssrv.MakeReplMemFs(addr, "", "named", nil)
	}

	if err != nil {
		db.DFatalf("%v: err %v\n", proc.GetProgram(), err)
	}

	// seccomp.LoadFilter()

	initfs(ss)

	db.DPrintf(db.NAMED, "Named started rid:%v ip:%v", realmId, ss.MyAddr())

	ss.Serve()
	ss.Done()
}

var InitDir = []string{sp.TMPREL, sp.BOOTREL, sp.KPIDSREL, sp.SCHEDDREL, sp.PROCDREL, sp.UXREL, sp.S3REL, sp.DBREL, sp.REALMS, sp.HOTELREL, sp.CACHEREL, sp.WS_REL, sp.WS_RUNQ_LC_REL, sp.WS_RUNQ_BE_REL}

func initfs(ss *sesssrv.SessSrv) error {
	r := ss.Root()
	for _, n := range InitDir {
		_, err := r.Create(ctx.MkCtx("", 0, nil), n, 0777|sp.DMDIR, sp.OREAD)
		if err != nil {
			return err
		}
	}
	return nil
}
