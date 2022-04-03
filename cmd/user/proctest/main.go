package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	M = 20
)

func BurstProc(n int, f func(chan error)) error {
	ch := make(chan error)
	for i := 0; i < n; i++ {
		go f(ch)
	}
	var err error
	for i := 0; i < n; i++ {
		r := <-ch
		if r != nil && err == nil {
			err = r
		}
	}
	return err
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <n> <program> <program-args>\n", os.Args[0])
		os.Exit(1)
	}
	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "n is not a number %v\n", os.Args[1])
		os.Exit(1)
	}

	fsl := fslib.MakeFsLib(os.Args[0] + "-" + proc.GetPid().String())
	pclnt := procclnt.MakeProcClnt(fsl)
	err = pclnt.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	start := time.Now()
	for i := 0; i < n; i += M {
		if i%1000 == 0 {
			log.Printf("i = %d %dms\n", i, time.Since(start).Milliseconds())
			start = time.Now()
		}
		err := BurstProc(M, func(ch chan error) {
			a := proc.MakeProc(os.Args[2], os.Args[3:])
			if err := pclnt.Spawn(a); err != nil {
				ch <- err
				return
			}
			if err := pclnt.WaitStart(a.Pid); err != nil {
				ch <- err
				return
			}
			status, err := pclnt.WaitExit(a.Pid)
			if err != nil {
				ch <- err
				return
			}
			if !status.IsStatusOK() {
				ch <- fmt.Errorf("status error %v", status.Error())
				return
			}
			ch <- nil

		})
		if err != nil {
			pclnt.Exited(proc.MakeStatusErr(err.Error(), nil))
			os.Exit(1)
		}
	}
	pclnt.Exited(proc.MakeStatus(proc.StatusOK))
}
