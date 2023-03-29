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
	if len(os.Args) != 5 {
		db.DFatalf("Usage: %v id delay mem duration\nArgs: %v", os.Args[0], os.Args)
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
	dur, err := time.ParseDuration(os.Args[4])
	if err != nil {
		db.DFatalf("Error ParseDuration: %v", err)
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
	db.DPrintf(db.ALWAYS, "%v: start %v %v %v", id, d, humanize.Bytes(m), dur)
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
	for time.Since(t) < dur {
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
	for i := uint64(0); i < m; i += uint64(sp.KBYTE) {
		k := j * i
		j = k + i
		mem[j%m] = mem[k%m] + mem[i%m] + byte(i%8)
	}
	return m
}
