package crash

import (
	"os"
	"strconv"
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

func GetEnv(name string) int64 {
	crash := os.Getenv(name)
	n, err := strconv.Atoi(crash)
	if err != nil {
		n = 0
	}
	return int64(n)
}

func randSleep(c int64) uint64 {
	ms := rand.Int64(c)
	db.DPrintf(db.CRASH, "randSleep %dms\n", ms)
	time.Sleep(time.Duration(ms) * time.Millisecond)
	r := rand.Int64(1000)
	return r
}

func Crasher(fsl *fslib.FsLib) {
	crash := GetEnv(proc.SIGMACRASH)
	if crash == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(crash)
			if r < 330 {
				Crash()
			} else if r < 660 {
				PartitionNamed(fsl)
			}
		}
	}()
}

func Partitioner(ss *sesssrv.SessSrv) {
	crash := GetEnv(proc.SIGMAPARTITION)
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
	crash := GetEnv(proc.SIGMANETFAIL)
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

func PartitionParentProb(sc *sigmaclnt.SigmaClnt, prob uint64) bool {
	crash := GetEnv(proc.SIGMACRASH)
	if crash == 0 {
		return false
	}
	p := rand.Int64(100)
	if p < prob {
		db.DPrintf(db.ALWAYS, "PartitionParentProb %v\n", prob)
		sc.ProcClnt.Exited(proc.MakeStatusErr("partitioned", nil))
		return true
	}
	return false
}

func Crash() {
	db.DPrintf(db.ALWAYS, "crash.Crash %v\n", GetEnv(proc.SIGMACRASH))
	os.Exit(1)
}

func PartitionNamed(fsl *fslib.FsLib) {
	db.DPrintf(db.ALWAYS, "crash.Partition %v\n", GetEnv(proc.SIGMAPARTITION))
	if error := fsl.Disconnect(sp.NAMED); error != nil {
		db.DPrintf(db.ALWAYS, "Disconnect %v name fails err %v\n", os.Args, error)
	}
}

func MaybePartition(fsl *fslib.FsLib) bool {
	r := rand.Int64(1000)
	if r < 330 {
		PartitionNamed(fsl)
		return true
	}
	return false
}
