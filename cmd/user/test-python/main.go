package main

import (
	"log"
	"os"

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
	db.DPrintf(db.TEST, "running %v\n", os.Args)

	p, err := os.StartProcess("/bin2/python3", append([]string{"/bin2/python3", "/bin2/hello.py"}), &os.ProcAttr{})
	if err != nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "exec failed", err))
	}
	s, err := p.Wait()
	if err != nil {
		sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "wait failed", err))
	}

	db.DPrintf(db.TEST, "Wait returns %v\n", s)

	sc.ClntExitOK()
}
