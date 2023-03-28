package main

import (
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/process"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
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
	mem := make([]byte, m)
	iter := uint64(0)
	for time.Since(t) < time.Duration(5*time.Second) {
		iter += rw(mem, m)
	}
	pf1, err := proc.PageFaults()
	if err != nil {
		db.DFatalf("Error PageFaults: %v", err)
	}
	db.DPrintf(db.ALWAYS, "%v: done %v %v %v %v", id, iter, time.Since(t), pf, pf1)
	if id == "BE" {
		time.Sleep(d)
	}
	sc.ExitedOK()
}

func rw(mem []byte, m uint64) uint64 {
	j := uint64(0)
	ps := uint64(4 * sp.KBYTE)
	for i := uint64(0); i < m; i += ps {
		k := j * i
		j = k + i
		mem[j%m] = mem[k%m] + mem[i%m]
	}
	return m
}
