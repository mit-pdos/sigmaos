package main

import (
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"

	"time"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <n> <sleep_length>\n", os.Args[0])
		os.Exit(1)
	}

	// sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	// if err != nil {
	// 	sc.ClntExit(proc.NewStatus(proc.StatusErr))
	// }
	// err = sc.Started()
	// if err != nil {
	// 	sc.ClntExit(proc.NewStatus(proc.StatusErr))
	// }

	// timer := time.NewTicker(5 * time.Second)

	// <-timer.C

	db.DPrintf(db.ALWAYS, "Set started")

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt: error %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}

	timer := time.NewTicker(5 * time.Second)

	testDir := sp.S3 + "~any/hmngtestbucket/"
	filePath := testDir + "example-out.txt"
	dstFd, err := sc.Create(filePath, 0777, sp.OWRITE)
	if err != nil {
		db.DFatalf("Error creating out file in s3 %v\n", err)
	}

	for {
		select {
		case <-timer.C:
			sc.Write(dstFd, []byte("exiting"))
			err = sc.Close(dstFd)
			sc.ClntExitOK()
			return
		default:
			fmt.Println("here sleep")
			sc.Write(dstFd, []byte("here sleep"))
			time.Sleep(2 * time.Second)
		}
	}
}
