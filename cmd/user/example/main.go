package main

import (
	"fmt"
	"os"
	"strconv"
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

	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		db.DFatalf("strconv err %v", err)
	}

	d, err := time.ParseDuration(os.Args[2])
	if err != nil {
		db.DFatalf("Error parsing duration: %v", err)
	}

	db.DPrintf(db.ALWAYS, "Running %v", os.Args)

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("example %v", err)
	}

	db.DPrintf(db.ALWAYS, "Set started")

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
}
