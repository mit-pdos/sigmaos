package main

import (
	"os"
	"time"

	"github.com/dustin/go-humanize"
        "github.com/shirou/gopsutil/process"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func main() {
	if len(os.Args) != 4 {
		db.DFatalf("Usage: %v id delay mem\nArgs: %v", os.Args[0], os.Args)
	}
	id := os.Args[1]
	d, err := time.ParseDuration(os.Args[2])
	if err != nil {
		db.DFatalf("Error ParseDuration: %v", err)
	}
	m, err := humanize.ParseBytes(os.Args[3])
	if err != nil {
		db.DFatalf("Error ParseBytes: %v", err)
	}
		sc, err := sigmaclnt.MkSigmaClnt("memhog-" + proc.GetPid().String())
		if err != nil {
			db.DFatalf("Error mkSigmaClnt: %v", err)
		}
		if err := sc.Started(); err != nil {
			db.DFatalf("Error started: %v", err)
		}
	if id == "LC" {
		time.Sleep(d)
	}
	db.DPrintf(db.ALWAYS, "%v:  start (%v %v)", id, d, m)
	pid := os.Getpid()
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
	    db.DFatalf("Error NewProcess: %v", err)
	}
	t := time.Now()
	pf, err := proc.PageFaults()
	if err != nil {
	    db.DFatalf("Error PageFaults: %v", err)
	}
	mem := make([]byte, 1)
	if m > 0 {
		mem = make([]byte, m)
	}
	j := uint(0)
	niter := uint(10_000_000)
	for i := uint(0); i < niter; i++ {
		k := j * i
		j = k + i
		l := uint(len(mem))
		mem[j%l] = mem[k%l] + mem[i%l]
	}
	pf1, err:= proc.PageFaults()
	if err != nil {
	    db.DFatalf("Error PageFaults: %v", err)
	}
	db.DPrintf(db.ALWAYS, "%v: iter %v %v %v %v", id, niter,  time.Since(t), pf, pf1)
	if id == "BE" {
	    time.Sleep(d)
	}
	sc.ExitedOK()
}

