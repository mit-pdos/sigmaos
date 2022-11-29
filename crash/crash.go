package crash

import (
	"os"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	np "sigmaos/sigmap"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sesssrv"
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
				Crash(fsl)
			} else if r < 660 {
				Partition(fsl)
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

func Crash(fsl *fslib.FsLib) {
	db.DPrintf(db.ALWAYS, "crash.Crash %v\n", os.Args)
	os.Exit(1)
}

func Partition(fsl *fslib.FsLib) {
	db.DPrintf(db.ALWAYS, "crash.Partition %v\n", os.Args)
	if error := fsl.Disconnect(np.NAMED); error != nil {
		db.DPrintf(db.ALWAYS, "Disconnect %v name fails err %v\n", os.Args, error)
	}
}

func MaybePartition(fsl *fslib.FsLib) bool {
	r := rand.Int64(1000)
	if r < 330 {
		Partition(fsl)
		return true
	}
	return false
}
