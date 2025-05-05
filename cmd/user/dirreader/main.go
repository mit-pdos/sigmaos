package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v dir\n", os.Args[0])
	}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	if err := sc.Started(); err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	sts, err := sc.GetDir(path.MarkResolve(os.Args[1]))
	if err != nil {
		sc.ClntExit(proc.NewStatusErr(err.Error(), nil))
	} else {
		db.DPrintf(db.ALWAYS, "%v sts %v", os.Args[1], sp.Names(sts))
		sc.ClntExit(proc.NewStatusInfo(proc.StatusOK, "GetDir", sp.Names(sts)))
	}
	os.Exit(0)
}
