package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"

	"time"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v <runtime> <npages> <ckpt-pn>\n", os.Args[0])
		os.Exit(1)
	}
	sec, err := strconv.Atoi(os.Args[1])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}
	npages, err := strconv.Atoi(os.Args[2])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}
	ckptpn := os.Args[3]

	db.DPrintf(db.ALWAYS, "Running %d %d %v", sec, npages, ckptpn)

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt error %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started error %v\n", err)
	}

	timer := time.NewTicker(time.Duration(sec) * time.Second)

	pagesz := os.Getpagesize()
	mem := make([]byte, pagesz*npages)
	for i := 0; i < npages; i++ {
		mem[i*pagesz] = byte(i)
	}

	_, err = sc.Stat(sp.UX + "~any/")
	if err != nil {
		db.DFatalf("Stat err %v\n", err)
	}

	sc, err = sc.CheckpointMe(ckptpn)
	if err != nil {
		db.DFatalf("Checkpoint me didn't return error", err)
	}

	db.DPrintf(db.ALWAYS, "Mark started")
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started error %v\n", err)
	}

	for {
		select {
		case <-timer.C:
			db.DPrintf(db.ALWAYS, "ClntExit")
			sc.ClntExitOK()
			return
		default:
			r := rand.IntN(npages)
			db.DPrintf(db.ALWAYS, "Write page %d", r)
			mem[r*pagesz] = byte(r)
			time.Sleep(1 * time.Second)
		}
	}
}
