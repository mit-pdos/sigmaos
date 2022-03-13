package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"ulambda/delay"
	"ulambda/fenceclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	N     = 10
	DELAY = 10
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "%v: Usage: <fence> <dir> <last>\n", os.Args[0])
		os.Exit(1)
	}
	pid := proc.GetPid()
	fsl := fslib.MakeFsLib("primary-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)

	pclnt.Started(pid)

	dir := os.Args[2]
	fn := dir + "/out"
	f := fenceclnt.MakeFenceClnt(fsl, os.Args[1], 0, []string{dir})

	err := f.AcquireFenceW([]byte(pid))
	if err != nil {
		log.Fatalf("FATAL %v AcquireFenceW %v failed %v\n", pid, os.Args[1], err)
	}

	log.Printf("%v: primary %v\n", proc.GetName(), os.Args)

	b, err := json.Marshal(pid)
	if err != nil {
		log.Fatalf("FATAL %v marshal %v failed %v\n", pid, fn, err)
	}
	lb := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(lb, int64(len(b)))
	_, err = fsl.SetFile(fn, lb[0:n], np.NoOffset)
	if err != nil {
		log.Fatalf("FATAL %v SetFile lb %v failed %v\n", pid, fn, err)
	}
	_, err = fsl.SetFile(fn, b, np.NoOffset)
	if err != nil {
		log.Fatalf("FATAL %v SetFile b %v failed %v\n", pid, fn, err)
	}

	if os.Args[3] == "last" {
		// allow others to write for a while
		time.Sleep(500 * time.Millisecond)
	} else {
		fsl.Disconnect(os.Args[1])

		// wait a little before starting to write
		time.Sleep(10 * time.Millisecond)

		// and delay writes
		delay.Delay(DELAY)

		for i := 0; i < N; i++ {
			_, err := fsl.SetFile(fn, []byte(pid), np.NoOffset)
			if err != nil {
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			}
		}
	}

	pclnt.Exited(pid, proc.MakeStatus(proc.StatusOK))
}
