package main

import (
	"log"
	"os"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v dir\n", os.Args[0])
	}
	sc, err := sigmaclnt.NewSigmaClnt(sp.Tuname(os.Args[0]))
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	if err := sc.Started(); err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	sts, err := sc.GetDir(os.Args[1] + "/")
	if err != nil {
		sc.ClntExit(proc.MakeStatusErr(err.Error(), nil))
	} else {
		log.Printf("%v sts %v\n", os.Args[1], sp.Names(sts))
		sc.ClntExit(proc.MakeStatusInfo(proc.StatusOK, "GetDir", sp.Names(sts)))
	}
	os.Exit(0)
}
