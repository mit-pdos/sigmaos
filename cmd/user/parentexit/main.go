package main

import (
	"fmt"
	"os"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procinit"
)

//
// Parent creates a child proc but parent exits before child exits
//

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "%v: Usage msec pid\n", os.Args[0])
		os.Exit(1)
	}
	fsl := fslib.MakeFsLib(os.Args[0])
	pclnt := procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap())
	pid1 := os.Args[2]
	a := proc.MakeProc(pid1, "bin/user/sleeper", []string{os.Args[1], "name/out_" + pid1})
	err := pclnt.Spawn(a)
	if err != nil {
		pclnt.Exited(proc.GetPid(), err.Error())
	}
	err = pclnt.WaitStart(pid1)
	if err != nil {
		pclnt.Exited(proc.GetPid(), err.Error())
	}
	pclnt.Exited(proc.GetPid(), "OK")
}
