package main

import (
	"os"
	"os/user"
	"syscall"

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
	db.DPrintf(db.TEST, "running %v\n", os.Args)
	if _, err := user.Current(); err == nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "getuid succeeded", nil))
	}
	if err := syscall.Chroot("/"); err == nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "chroot succeeded", nil))
	}
	if _, err := os.StartProcess("/bin2/uprocd", append([]string{"/"}), &os.ProcAttr{}); err == nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "exec succeeded", nil))
	} else {
		db.DPrintf(db.TEST, "exec err %v\n", err)
	}
	sc.ClntExitOK()
}
