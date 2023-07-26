package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/seqwc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname(os.Args[0] + "-" + proc.GetPid().String()))
	if err != nil {
		db.DFatalf("MkSigmaClnt: error %v\n", err)
	}
	p, err := perf.MakePerf(perf.SEQWC)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
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
	sc.ClntExit(proc.MakeStatusInfo(proc.StatusOK, strconv.Itoa(n), nil))
}
