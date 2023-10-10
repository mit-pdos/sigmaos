package main

import (
	"log"
	"os"
	"os/exec"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func printDir(dir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		db.DPrintf(db.TEST, "entry %v/%v\n", dir, file.Name())
	}
}

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClient: %v", err)
	}
	if err := sc.Started(); err != nil {
		db.DFatalf("Started err %v", err)
	}
	db.DPrintf(db.TEST, "Running %v\n", os.Args)

	cmd := exec.Command("/bin2/python3", append([]string{"/bin2/hello.py"})...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "run failed", err))
	}
	// err.(*exec.ExitError).Sys().(syscall.WaitStatus).ExitStatus()
	db.DPrintf(db.TEST, "Run returned ok \n")
	sc.ClntExitOK()
}
