package main

import (
	// "fmt"
	"os"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/process"

	"sigmaos/proc"
	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) != 6 {
		db.DFatalf("Usage: %v id delay mem duration nthread\nArgs: %v", os.Args[0], os.Args)
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
	nthread, err := strconv.Atoi(os.Args[5])
	if err != nil {
		db.DFatalf("Error strconv: %v", err)
	}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error mkSigmaClnt: %v", err)
	}
	if err := sc.Started(); err != nil {
		db.DFatalf("Error started: %v", err)
	}
	if id == "LC" {
		time.Sleep(d)
	}
	db.DPrintf(db.ALWAYS, "%v: start %v %v %v %d %d", id, d, humanize.Bytes(m), dur, nthread, linuxsched.NCores)
	pid := os.Getpid()
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		db.DFatalf("Error NewProcess: %v", err)
	}
	pf, err := proc.PageFaults()
	if err != nil {
		db.DFatalf("Error PageFaults: %v", err)
	}
	iter := uint64(0)
	ch := make(chan uint64)
	t := time.Now()
	for i := 0; i < nthread; i++ {
		go worker(ch, m/uint64(nthread), t, dur)
	}
	for i := 0; i < nthread; i++ {
		iter += <-ch
	}
	pf1, err := proc.PageFaults()
	if err != nil {
		db.DFatalf("Error PageFaults: %v", err)
	}
	tput := float64(iter) / time.Since(t).Seconds()
	db.DPrintf(db.ALWAYS, "%v: done dur %v iter %v tpt %.2fMOps/sec pf-pre %v pf-post %v", id, time.Since(t), iter, tput/1_000_000, pf, pf1)
	if id == "BE" {
		time.Sleep(d)
	}
	sc.ClntExitOK()
}

func worker(ch chan uint64, m uint64, t time.Time, dur time.Duration) {
	mem := make([]byte, m)
	iter := uint64(0)
	for time.Since(t) < dur {
		iter += rw(mem)
	}
	ch <- iter
}

func rw(mem []byte) uint64 {
	j := uint64(0)
	l := uint64(len(mem))
	//start := time.Now()
	inc := uint64(sp.KBYTE)
	for i := uint64(0); i < l; i += inc {
		k := j * i
		j = k + i
		mem[j%l] = mem[k%l] + mem[i%l] + byte(i%8)
	}
	//fmt.Printf("time %v\n", time.Since(start))
	return l / inc
}
