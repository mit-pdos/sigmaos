package crash

import (
	"os"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Crash/partition testing
//

func randSleep(c int64) uint64 {
	ms := rand.Int64(c)
	db.DPrintf(db.CRASH, "randSleep %dms\n", ms)
	time.Sleep(time.Duration(ms) * time.Millisecond)
	r := rand.Int64(1000)
	return r
}

func Crasher(fsl *fslib.FsLib) {
	crash := fsl.ProcEnv().GetCrash()
	if crash == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(crash)
			if r < 330 {
				Crash(crash)
			} else if r < 660 {
				PartitionNamed(fsl)
			}
		}
	}()
}

func Partitioner(ss *sesssrv.SessSrv) {
	crash := ss.ProcEnv().GetPartition()
	if crash == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(crash)
			if r < 330 {
				ss.PartitionClient(true)
			}
		}
	}()
}

func NetFailer(ss *sesssrv.SessSrv) {
	crash := ss.ProcEnv().GetNetFail()
	if crash == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(crash)
			if r < 330 {
				ss.PartitionClient(false)
			}
		}
	}()
}

// Randomly tell parent we exited but then keep running, simulating a
// network partition from the parent's point of view.
func PartitionParentProb(sc *sigmaclnt.SigmaClnt, prob uint64) bool {
	crash := sc.ProcEnv().GetCrash()
	if crash == 0 {
		return false
	}
	p := rand.Int64(100)
	if p < prob {
		db.DPrintf(db.ALWAYS, "PartitionParentProb %v\n", prob)
		sc.ProcAPI.Exited(proc.NewStatusErr("partitioned", nil))
		return true
	}
	return false
}

func Crash(crash int64) {
	db.DPrintf(db.ALWAYS, "crash.Crash %v\n", crash)
	os.Exit(1)
}

func PartitionNamed(fsl *fslib.FsLib) {
	db.DPrintf(db.ALWAYS, "crash.Partition %v\n", fsl.ProcEnv().GetPartition())
	if error := fsl.Disconnect(sp.NAMED); error != nil {
		db.DPrintf(db.ALWAYS, "Disconnect %v name fails err %v\n", os.Args, error)
	}
}
