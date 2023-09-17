package main

import (
	"fmt"
	"os"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Parent creates a child proc but parent exits before child exits
//

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "%v: Usage msec pid\n", os.Args[0])
		os.Exit(1)
	}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt err %v\n", err)
	}
	sc.Started()
	pid1 := sp.Tpid(os.Args[2])
	a := proc.NewProcPid(pid1, "sleeper", []string{os.Args[1], "name/"})
	err = sc.Spawn(a)
	if err != nil {
		sc.ClntExit(proc.NewStatusErr(err.Error(), nil))
	}
	err = sc.WaitStart(pid1)
	if err != nil {
		sc.ClntExit(proc.NewStatusErr(err.Error(), nil))
	}
	sc.ClntExitOK()
}
