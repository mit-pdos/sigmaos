package named

import (
	"log"
	"strconv"
	"strings"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/fs"
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
	realmId := sp.Trealm(args[2])
	pn := args[3]

	db.DPrintf(db.NAMED, "Named starting rid:%v pn %v addr:%v\n", realmId, pn, addr)

	var ss *sesssrv.SessSrv
	var err *serr.Err

	// Replicate?
	if len(args) >= 5 {
		var config repl.Config = nil
		if args[3] == "dummy" {
			config = repldummy.MakeConfig()
		} else {
			id, r := strconv.Atoi(args[4])
			if r != nil {
				db.DFatalf("Couldn't convert id string: %v", err)
			}
			peers := strings.Split(args[5], ",")
			config = replraft.MakeRaftConfig(id, peers, true)
		}
		ss, err = memfssrv.MakeReplMemFs(addr, pn, "named", config)
	} else {
		ss, err = memfssrv.MakeReplMemFs(addr, pn, "named", nil)
	}

	if err != nil {
		db.DFatalf("%v: err %v\n", proc.GetProgram(), err)
	}

	// seccomp.LoadFilter()

	if realmId == sp.ROOTREALM {
		initfs(ss, InitRootDir)
		makews(ss)
	} else {
		initfs(ss, InitDir)
	}

	sc := ss.SigmaClnt()
	db.DPrintf(db.NAMED, "Named started rid:%v pn %v addr:%v named %v", realmId, pn, ss.MyAddr(), sc.NamedAddr())

	ss.Serve()
	ss.Done()
}

var InitRootDir = []string{sp.TMPREL, sp.BOOTREL, sp.KPIDSREL, sp.SCHEDDREL, sp.UXREL, sp.S3REL, sp.DBREL, sp.HOTELREL, sp.CACHEREL}

var InitDir = []string{sp.TMPREL, sp.HOTELREL, sp.CACHEREL}

func initfs(ss *sesssrv.SessSrv, root []string) error {
	r := ss.Root()
	for _, n := range root {
		_, err := r.Create(ctx.MkCtx("", 0, nil), n, 0777|sp.DMDIR, sp.OREAD)
		if err != nil {
			db.DFatalf("Error create [%v]: %v", n, err)
			return err
		}
	}
	return nil
}

func makews(ss *sesssrv.SessSrv) error {
	r := ss.Root()
	ws, err := r.Create(ctx.MkCtx("", 0, nil), sp.WS_REL, 0777|sp.DMDIR, sp.OREAD|sp.OWRITE)
	if err != nil {
		db.DFatalf("Error create [%v]: %v", sp.WS_REL, err)
		return err
	}
	for _, n := range []string{sp.WS_RUNQ_LC_REL, sp.WS_RUNQ_BE_REL} {
		_, err := ws.(fs.Dir).Create(ctx.MkCtx("", 0, nil), n, 0777|sp.DMDIR, sp.OREAD)
		if err != nil {
			db.DFatalf("Error create [%v]: %v", n, err)
			return err
		}
	}
	return nil
}
