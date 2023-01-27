package main

import (
	"log"
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/seqgrep"
	"sigmaos/sigmaclnt"
)

func main() {
	sc, err := sigmaclnt.MkSigmaClnt(os.Args[0] + "-" + proc.GetPid().String())
	if err != nil {
		db.DFatalf("MkSigmaClnt: error %v\n", err)
	}
	p, err := perf.MakePerf(perf.SEQGREP)
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
	n := seqgrep.Grep(rdr)
	log.Printf("n = %d\n", n)
	p.Done()
	sc.Exited(proc.MakeStatusInfo(proc.StatusOK, strconv.Itoa(n), nil))
}
