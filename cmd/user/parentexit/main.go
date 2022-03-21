package main

import (
	"fmt"
	"os"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Parent creates a child proc but parent exits before child exits
//

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "%v: Usage msec pid\n", os.Args[0])
		os.Exit(1)
	}
	fsl := fslib.MakeFsLib(os.Args[0] + "-" + proc.GetPid().String())
	pclnt := procclnt.MakeProcClnt(fsl)
	pclnt.Started(proc.GetPid())
	pid1 := proc.Tpid(os.Args[2])
	a := proc.MakeProcPid(pid1, "bin/user/sleeper", []string{os.Args[1], "name/out_" + pid1.String()})
	err := pclnt.Spawn(a)
	if err != nil {
		pclnt.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
	}
	err = pclnt.WaitStart(pid1)
	if err != nil {
		pclnt.Exited(proc.GetPid(), proc.MakeStatusErr(err.Error(), nil))
	}
	pclnt.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
}
