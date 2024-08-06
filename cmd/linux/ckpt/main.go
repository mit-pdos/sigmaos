package main

import (
	"fmt"
	"os"
	"strconv"

	db "sigmaos/debug"

	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <sleep_length>\n", os.Args[0])
		os.Exit(1)
	}

	db.DPrintf(db.ALWAYS, "Pid: %d", os.Getpid())

	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("Atoi err %v\n", err)
		return
	}
	timer := time.NewTicker(time.Duration(n) * time.Second)

	f, err := os.Create("/tmp/ckpt")
	if err != nil {
		db.DFatalf("Error creating %v\n", err)
	}

	for {
		select {
		case <-timer.C:
			f.Write([]byte("exiting"))
			return
		default:
			fmt.Println("here sleep")
			f.Write([]byte("here sleep"))
			time.Sleep(2 * time.Second)
		}
	}
}
