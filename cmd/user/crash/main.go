// Crash or partition proc
package main

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
)

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt err %v\n", err)
	}
	err = sc.Started()
	if err != nil {
		db.DFatalf("Started: err %v\n", err)
	}
	_, err = sc.GetDir(sp.NAMED)
	if err != nil {
		db.DFatalf("Named GetDir error: %v", err)
	}
	crash.Failer(sc.FsLib, crash.CRASH_CRASH, func(e crash.Tevent) {
		crash.Crash()
	})
	crash.Failer(sc.FsLib, crash.CRASH_PARTITION, func(e crash.Tevent) {
		crash.PartitionAll(sc.FsLib)
	})

	time.Sleep(1000 * time.Millisecond)

	// This exit will not mark proc as exited if proc disconnected.
	sc.ClntExitOK()
}
