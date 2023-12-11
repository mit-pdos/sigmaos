package main

import (
	"fmt"
<<<<<<< HEAD
	"os"
	"strconv"
=======
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
>>>>>>> chkpt-rest
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <n> <sleep_length>\n", os.Args[0])
		os.Exit(1)
	}

<<<<<<< HEAD
	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		db.DFatalf("strconv err %v", err)
	}

	d, err := time.ParseDuration(os.Args[2])
	if err != nil {
		db.DFatalf("Error parsing duration: %v", err)
	}

	db.DPrintf(db.ALWAYS, "Running %v", os.Args)
=======
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
>>>>>>> chkpt-rest

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("example %v", err)
	}

	db.DPrintf(db.ALWAYS, "Set started")

<<<<<<< HEAD
	if err := sc.Started(); err != nil {
		db.DFatalf("Started err %v", err)
	}

	for i := 1; i < n; i++ {
		db.DPrintf(db.ALWAYS, "Running ..")
		time.Sleep(d)
		f, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			os.Exit(1)
		}
		fmt.Printf(".")
		if _, err := f.WriteString("Running..\n"); err != nil {
			os.Exit(1)
		}
		f.Close()
	}

	db.DPrintf(db.ALWAYS, "Exit")

	sc.ClntExitOK()
=======
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
>>>>>>> chkpt-rest
}
