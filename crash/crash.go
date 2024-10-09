// The crash package is used by procs to randomly crash and
// introduce permanant/temporary network partitions.
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

func randSleep(c int64) uint64 {
	ms := rand.Int64(c)
	r := rand.Int64(1000)
	db.DPrintf(db.CRASH, "randSleep %dms r %d\n", ms, r)
	time.Sleep(time.Duration(ms) * time.Millisecond)
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
				fail(crash, nil)
			} else if r < 660 {
				partitionNamed(fsl)
			}
		}
	}()
}

func CrasherMsg(fsl *fslib.FsLib, f func() string) {
	crash := fsl.ProcEnv().GetCrash()
	if crash == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(crash)
			if r < 330 {
				fail(crash, f)
			} else if r < 660 {
				partitionNamed(fsl)
			}
		}
	}()
}

func Partitioner(ss *sesssrv.SessSrv) {
	part := ss.ProcEnv().GetPartition()
	if part == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(part)
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
		db.DPrintf(db.CRASH, "PartitionParentProb %v p %v\n", prob, p)
		sc.ProcAPI.Exited(proc.NewStatusErr("partitioned", nil))
		return true
	}
	return false
}

func fail(crash int64, f func() string) {
	msg := ""
	if f != nil {
		msg = f()
	}
	db.DPrintf(db.CRASH, "crash.fail %v %v\n", crash, msg)
	os.Exit(1)
}

func Fail(crash int64) {
	fail(crash, nil)
}

func partitionNamed(fsl *fslib.FsLib) {
	db.DPrintf(db.CRASH, "crash.Partition from %v\n", sp.NAMED)
	if error := fsl.Disconnect(sp.NAMED); error != nil {
		db.DPrintf(db.CRASH, "Disconnect %v name fails err %v\n", os.Args, error)
	}
}
