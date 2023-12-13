package main

import (
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"time"
)

func main() {

	// sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	// if err != nil {
	// 	sc.ClntExit(proc.NewStatus(proc.StatusErr))
	// }
	// err = sc.Started()
	// if err != nil {
	// 	sc.ClntExit(proc.NewStatus(proc.StatusErr))
	// }

	// timer := time.NewTicker(5 * time.Second)

	// <-timer.C

	// sc.ClntExit(proc.NewStatus(proc.StatusOK))

	// ---------------

	sc, _ := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	sc.Started()

	timer := time.NewTicker(120 * time.Second)

	testDir := sp.S3 + "~any/hmngtestbucket/"
	filePath := testDir + "example-out.txt"
	dstFd, _ := sc.Create(filePath, 0777, sp.OWRITE)

	for {
		select {
		case <-timer.C:
			// fmt.Println("exiting")
			sc.Write(dstFd, []byte("exiting"))
			sc.Close(dstFd)
			sc.ClntExitOK()
			return
		default:
			// fmt.Println("here sleep")
			sc.Write(dstFd, []byte("here sleep"))
			time.Sleep(2 * time.Second)
		}
	}
}
