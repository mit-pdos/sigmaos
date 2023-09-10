package main

import (
	"log"
	"os"
	"strconv"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/seqgrep"
	"sigmaos/sigmaclnt"
)

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(config.GetProcEnv())
	if err != nil {
		db.DFatalf("MkSigmaClnt: error %v\n", err)
	}
	p, err := perf.MakePerf(sc.ProcEnv(), perf.SEQGREP)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	log.Printf("input: %s\n", os.Args[1])
	rdr, err := sc.OpenAsyncReader(os.Args[1], 0)
	if err != nil {
		db.DFatalf("OpenReader %v error %v\n", os.Args[1], err)
	}
	n := seqgrep.Grep(sc.ProcEnv(), rdr)
	log.Printf("n = %d\n", n)
	p.Done()
	sc.ClntExit(proc.MakeStatusInfo(proc.StatusOK, strconv.Itoa(n), nil))
}
