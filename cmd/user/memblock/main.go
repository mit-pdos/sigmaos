package main

import (
	"os"
	"path"

	"github.com/dustin/go-humanize"

	db "sigmaos/debug"
	linuxsched "sigmaos/util/linux/sched"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) != 2 {
		db.DFatalf("Usage: %v mem\nArgs: %v", os.Args[0], os.Args)
	}
	pe := proc.GetProcEnv()
	m, err := humanize.ParseBytes(os.Args[1])
	if err != nil {
		db.DFatalf("Error ParseBytes: %v", err)
	}
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error newSigmaClnt: %v", err)
	}
	// Make the memblock dir.
	if err := sc.MkDir(sp.MEMBLOCK, 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		db.DFatalf("Unexpected mkdir err: %v", err)
	}
	// Register this memblocker.
	if _, err := sc.Create(path.Join(sp.MEMBLOCK, sc.ProcEnv().KernelID), 0777, 0); err != nil {
		db.DFatalf("Unexpected putfile err: %v", err)
	}
	db.DPrintf(db.ALWAYS, "Allocating %v bytes of memory", m)
	nthread := int(linuxsched.GetNCores())
	ch := make(chan []byte)
	// Allocate and write memory in parallel, to force OS allocation.
	for i := 0; i < nthread; i++ {
		go worker(ch, m/uint64(nthread))
	}
	bs := make([][]byte, 0, nthread)
	for i := 0; i < nthread; i++ {
		b := <-ch
		bs = append(bs, b)
	}
	// Mark that the memory has been blocked & started.
	if err := sc.Started(); err != nil {
		db.DFatalf("Error started: %v", err)
	}
	if err := sc.WaitEvict(pe.GetPID()); err != nil {
		db.DFatalf("Err waitevict: %v", err)
	}
	sc.ClntExit(proc.NewStatus(proc.StatusEvicted))
}

func worker(ch chan []byte, m uint64) {
	mem := make([]byte, m)
	for i := range mem {
		mem[i] = byte(i*i + i - 1)
	}
	ch <- mem
}
