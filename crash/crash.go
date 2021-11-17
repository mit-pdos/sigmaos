package crash

import (
	"os"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/rand"
)

//
// Crash/partition testing
//

func Crasher(fsl *fslib.FsLib) {
	go func() {
		ms := rand.Int64(500)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		r := rand.Int64(1000)
		if r < 330 {
			Crash(fsl)
		} else if r < 660 {
			Partition(fsl)
		}

	}()
}

func Crash(fsl *fslib.FsLib) {
	db.DPrintf("%v: CRASH %v\n", db.GetName(), proc.GetPid())
	os.Exit(1)
}

func Partition(fsl *fslib.FsLib) {
	db.DPrintf("%v: PARTITION %v\n", db.GetName(), proc.GetPid())
	fsl.Disconnect("name")
}

func MaybePartition(fsl *fslib.FsLib) bool {
	r := rand.Int64(1000)
	if r < 330 {
		Partition(fsl)
		return true
	}
	return false
}
