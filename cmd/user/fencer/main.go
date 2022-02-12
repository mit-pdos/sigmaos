package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"ulambda/crash"
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
// fence tester
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
//  If fence doesn't work, then this invariant breaks (counter and A
//  and may have no relation).
//
//  If holder paritions, looses fence, but delayed writes to A,
//  then could violate this invariant
//
//  Note: we couldn't use version # as is, since they are per file,
//  and here we require atomicity across two files on different
//  servers.
//

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <partition?> <dir1> <dir2>\n", os.Args[0])
		os.Exit(1)
	}
	fsl := fslib.MakeFsLib("fencer-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)

	l := fenceclnt.MakeFenceClnt(fsl, os.Args[2]+"/fence", 0)

	cnt := os.Args[2] + "/cnt"
	A := os.Args[3] + "/A"

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

		// open A and then maybe partition from named

		fd, err := fsl.Open(A, np.OREAD|np.OWRITE)
		if err != nil {
			log.Fatalf("%v getfile %v failed %v\n", i, A, err)
		}

		if os.Args[1] == "YES" {
			if crash.MaybePartition(fsl) {
				log.Printf("%v: partition\n", proc.GetProgram())
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

		// log.Printf("%v: n %v n1 %v", proc.GetProgram(), n, n1)

		if n != n1 && n+1 != n1 {
			log.Printf("%v: Wrong n %v n1 %v", proc.GetProgram(), n, n1)
			pclnt.Exited(proc.GetPid(), proc.MakeStatusErr("Invariant violated"))
		}

		_, err = fsl.Write(fd, []byte(strconv.Itoa(n+1)))
		if err != nil {
			// most likely write failed with stale error, but
			// we cannot talk to named anymore to report that,
			// so just give up.
			log.Fatalf("write %v failed %v\n", A, err)
		}

		// the write may succeed because there is no new fence holder just yet
		fsl.Close(fd)

		if partitioned {
			err := l.ReleaseFence()
			if err != nil {
				log.Printf("%v release err %v\n", proc.GetProgram(), err)
			}
			break
		}

		_, err = fsl.SetFile(cnt, []byte(strconv.Itoa(n+1)))
		if err != nil {
			log.Fatalf("setfile %v failed %v\n", cnt, err)
		}

		l.ReleaseFence()
	}
	pclnt.Exited(proc.GetPid(), proc.MakeStatus(proc.StatusOK))
}
