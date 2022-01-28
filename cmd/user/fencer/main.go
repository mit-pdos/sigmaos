package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/delay"
	"ulambda/fenceclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	N     = 100
	DELAY = 10
)

//
// lock tester
//
// start:  cnt = 0, A = 0
//
// in loop:
//  - read file cnt, which contains a counter
//  - read A
//  - check A is counter+1 or counter
//  - write counter+1 to A
//  - write counter+1 to counter
//
//  invariant: A is counter or counter+1
//
//  If lock doesn't work, then this invariant breaks (counter and A
//  and may have no relation).
//
//  If holder paritions, looses lock, and but delayed writes to A,
//  then could violate this invariant
//
//  Note: we couldn't use version # as is, since they are per file,
//  and here we require atomicity across different files.
//

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <partition?> <dir>\n", os.Args[0])
		os.Exit(1)
	}
	fsl := fslib.MakeFsLib("fencer-" + proc.GetPid())

	pclnt := procclnt.MakeProcClnt(fsl)

	l := fenceclnt.MakeFenceClnt(fsl, fenceclnt.FENCE_DIR+"/fence", 0)

	cnt := fenceclnt.FENCE_DIR + "/cnt"
	A := os.Args[2] + "/A"

	pclnt.Started(proc.GetPid())

	partitioned := false
	for i := 0; i < N; i++ {
		err := l.AcquireFenceW([]byte{})

		b, err := fsl.GetFile(cnt)
		if err != nil {
			log.Fatalf("getfile %v failed %v\n", cnt, err)
		}

		b1, err := fsl.GetFile(A)
		if err != nil {
			log.Fatalf("%v getfile %v failed %v\n", i, A, err)
		}

		// open A and then partition

		fd, err := fsl.Open(A, np.OREAD|np.OWRITE)
		if err != nil {
			log.Fatalf("%v getfile %v failed %v\n", i, A, err)
		}

		if os.Args[1] == "YES" {
			if crash.MaybePartition(fsl) {
				log.Printf("%v: partition\n", db.GetName())
				partitioned = true
			}
		}

		delay.Delay(DELAY)

		n, err := strconv.Atoi(string(b))
		if err != nil {
			log.Fatalf("strconv %v failed %v\n", cnt, err)
		}

		n1, err := strconv.Atoi(string(b1))
		if err != nil {
			log.Fatalf("strconv %v failed %v\n", A, err)
		}

		// log.Printf("%v: n %v n1 %v", db.GetName(), n, n1)

		if n != n1 && n+1 != n1 {
			log.Printf("%v: Wrong n %v n1 %v", db.GetName(), n, n1)
			pclnt.Exited(proc.GetPid(), "Invariant violated")
		}

		_, err = fsl.Write(fd, []byte(strconv.Itoa(n+1)))
		if err != nil {
			log.Fatalf("write %v failed %v\n", A, err)
		}

		fsl.Close(fd)

		if partitioned {
			err := l.ReleaseFence()
			if err != nil {
				log.Printf("%v unlock err %v\n", db.GetName(), err)
			}
			break
		}

		_, err = fsl.SetFile(cnt, []byte(strconv.Itoa(n+1)))
		if err != nil {
			log.Fatalf("setfile %v failed %v\n", cnt, err)
		}

		l.ReleaseFence()
	}
	pclnt.Exited(proc.GetPid(), "OK")
}
