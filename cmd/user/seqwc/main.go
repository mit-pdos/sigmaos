package main

import (
	"os"
	"strconv"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/seqwc"
)

func main() {
	fsl := fslib.MakeFsLib(os.Args[0] + "-" + proc.GetPid().String())
	pclnt := procclnt.MakeProcClnt(fsl)
	p := perf.MakePerf("SEQWC")
	err := pclnt.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	n, err := seqwc.Wc(fsl, os.Args[1])
	if err != nil {
		db.DFatalf("Wc: error %v\n", err)
	}
	p.Done()
	pclnt.Exited(proc.MakeStatusInfo(proc.StatusOK, strconv.Itoa(n), nil))
}
