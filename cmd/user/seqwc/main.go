package main

import (
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/seqwc"
)

func main() {
	fsl, err := fslib.MakeFsLib(os.Args[0] + "-" + proc.GetPid().String())
	if err != nil {
		db.DFatalf("MakeFsLib: error %v\n", err)
	}
	pclnt := procclnt.MakeProcClnt(fsl)
	p, err := perf.MakePerf("SEQWC")
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()
	err = pclnt.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	n, err := seqwc.Wc(fsl, os.Args[1], os.Args[2])
	if err != nil {
		db.DFatalf("Wc: error %v\n", err)
	}
	pclnt.Exited(proc.MakeStatusInfo(proc.StatusOK, strconv.Itoa(n), nil))
}
