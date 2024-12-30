package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
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
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v <n> <burst> <program> <program-args>\n", os.Args[0])
		os.Exit(1)
	}
	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "n is not a number %v\n", os.Args[1])
		os.Exit(1)
	}
	b, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "b is not a number %v\n", os.Args[2])
		os.Exit(1)
	}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt: error %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	start := time.Now()
	ncrash := 0
	for i := 0; i < n; i += b {
		if i%100 == 0 {
			db.DPrintf(db.ALWAYS, "iter i = %d %dms\n", i, time.Since(start).Milliseconds())
			start = time.Now()
		}
		err := BurstProc(b, func(ch chan error) {
			a := proc.NewProc(os.Args[3], os.Args[4:])
			db.DPrintf(db.TEST1, "Spawning %v", a.GetPid().String())
			if err := sc.Spawn(a); err != nil {
				ch <- err
				return
			}
			db.DPrintf(db.TEST1, "WaitStarting %v", a.GetPid().String())
			if err := sc.WaitStart(a.GetPid()); err != nil {
				ch <- err
				return
			}
			db.DPrintf(db.TEST1, "WaitExiting %v", a.GetPid().String())
			status, err := sc.WaitExit(a.GetPid())
			if err != nil {
				ch <- err
				return
			}
			db.DPrintf(db.TEST1, "Done %v %v", a.GetPid().String(), status)
			if !status.IsStatusOK() {
				ch <- status.Error()
				return
			}
			ch <- nil
		})

		if err != nil {
			sr := serr.NewErrString(err.Error())
			if !(os.Args[3] == "crash" && sr.Error() != proc.CRASHSTATUS) {
				sc.ClntExit(proc.NewStatusErr(sr.Error(), nil))
				os.Exit(1)
			}
			ncrash += 1
		}
	}
	sc.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK", ncrash))
}
