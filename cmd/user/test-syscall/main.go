package main

import (
	"bytes"
	"crypto/rand"
	"os"
	"os/user"
	"syscall"
	"time"

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

	now := time.Now()
	db.DPrintf(db.TEST, "Current date and time (RFC3339): %q\n", now.Format(time.RFC3339))

	c := 10
	b := make([]byte, c)
	if _, err := rand.Read(b); err != nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "rand failed", err))
	}
	// The slice should now contain random bytes instead of only zeroes.
	db.DPrintf(db.TEST, "bytes %v\n", bytes.Equal(b, make([]byte, c)))

	if _, err := user.Current(); err == nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "getuid succeeded", nil))
	}
	if err := syscall.Chroot("/"); err == nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "chroot succeeded", nil))
	}
	// size doesn't use /bin/busybox
	if _, err := os.StartProcess("/usr/bin/size", append([]string{"/usr/bin/size"}), &os.ProcAttr{}); err == nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "exec succeeded", nil))
	} else {
		db.DPrintf(db.TEST, "exec err %v\n", err)
	}
	sc.ClntExitOK()
}
