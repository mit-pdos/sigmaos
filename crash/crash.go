package crash

import (
	"log"
	"os"
	"strconv"
	"time"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/rand"
)

//
// Crash/partition testing
//

func GetEnv() int64 {
	crash := os.Getenv("SIGMACRASH")
	n, err := strconv.Atoi(crash)
	if err != nil {
		n = 0
	}
	return int64(n)
}

func Crasher(fsl *fslib.FsLib) {
	crash := GetEnv()
	if crash == 0 {
		return
	}
	go func() {
		for true {
			ms := rand.Int64(crash)
			// log.Printf("%v: ms %v\n", proc.GetProgram(), ms)
			time.Sleep(time.Duration(ms) * time.Millisecond)
			r := rand.Int64(1000)
			// log.Printf("%v: r = %v\n", proc.GetProgram(), r)
			if r < 330 {
				Crash(fsl)
			} else if r < 660 {
				Partition(fsl)
			}
		}
	}()
}

func Crash(fsl *fslib.FsLib) {
	log.Printf("%v: FAIL CRASH %v\n", proc.GetProgram(), proc.GetPid())
	os.Exit(1)
}

func Partition(fsl *fslib.FsLib) {
	log.Printf("%v: FAIL PARTITION %v\n", proc.GetProgram(), proc.GetPid())
	fsl.Disconnect("name")
	time.Sleep(time.Duration(5) * time.Millisecond)
}

func MaybePartition(fsl *fslib.FsLib) bool {
	r := rand.Int64(1000)
	if r < 330 {
		Partition(fsl)
		return true
	}
	return false
}
