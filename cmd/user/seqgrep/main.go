package main

import (
	"log"
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/seqgrep"
)

func main() {
	fsl, err := fslib.MakeFsLib(os.Args[0] + "-" + proc.GetPid().String())
	if err != nil {
		db.DFatalf("MakeFsLib: error %v\n", err)
	}
	pclnt := procclnt.MakeProcClnt(fsl)

	p, err := perf.MakePerf(perf.SEQGREP)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	err = pclnt.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	log.Printf("input: %s\n", os.Args[1])
	rdr, err := fsl.OpenAsyncReader(os.Args[1], 0)
	if err != nil {
		db.DFatalf("OpenReader %v error %v\n", os.Args[1], err)
	}
	n := seqgrep.Grep(rdr)
	log.Printf("n = %d\n", n)
	p.Done()
	pclnt.Exited(proc.MakeStatusInfo(proc.StatusOK, strconv.Itoa(n), nil))
}
