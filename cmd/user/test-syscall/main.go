package main

import (
	"os/user"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClient: %v", err)
	}
	if err := sc.Started(); err != nil {
		db.DFatalf("Started err %v", err)
	}
	if _, err := user.Current(); err == nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "getuid succeeded", nil))
	}
	sc.ClntExitOK()
}
