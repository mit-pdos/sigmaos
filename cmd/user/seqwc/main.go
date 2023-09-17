package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/seqwc"
	"sigmaos/sigmaclnt"
)

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("MkSigmaClnt: error %v\n", err)
	}
	p, err := perf.NewPerf(sc.ProcEnv(), perf.SEQWC)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	n, err := seqwc.Wc(sc.FsLib, os.Args[1], os.Args[2])
	if err != nil {
		db.DFatalf("Wc: error %v\n", err)
	}
	sc.ClntExit(proc.NewStatusInfo(proc.StatusOK, strconv.Itoa(n), nil))
}
