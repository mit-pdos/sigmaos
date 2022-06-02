package main

import (
	"bufio"
	"log"
	"os"
	"strconv"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/seqgrep"
)

func main() {
	fsl := fslib.MakeFsLib(os.Args[0] + "-" + proc.GetPid().String())
	pclnt := procclnt.MakeProcClnt(fsl)
	p := perf.MakePerf("SEQGREP")
	err := pclnt.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	log.Printf("input: %s\n", os.Args[1])
	r, err := fsl.OpenReader(os.Args[1])
	if err != nil {
		db.DFatalf("OpenReader %v error %v\n", os.Args[1], err)
	}
	rdr := bufio.NewReader(r)
	n := seqgrep.Grep(rdr)
	log.Printf("n = %d\n", n)
	p.Done()
	pclnt.Exited(proc.MakeStatusInfo(proc.StatusOK, strconv.Itoa(n), nil))
}
