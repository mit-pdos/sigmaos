package fenceclnttest

import (
	"log"
	"os"
	"time"

	"ulambda/delay"
	"ulambda/fenceclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/writer"
)

const (
	N     = 10
	DELAY = 10
)

func RunPrimary(fence, dir, last string) {
	pid := proc.GetPid()
	fsl := fslib.MakeFsLib("primary-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)

	pclnt.Started(pid)

	fn := dir + "/out"
	f := fenceclnt.MakeFenceClnt(fsl, os.Args[1], 0, []string{dir})

	err := f.AcquireFenceW([]byte(pid))
	if err != nil {
		log.Fatalf("FATAL %v AcquireFenceW %v failed %v\n", pid, os.Args[1], err)
	}

	log.Printf("%v: primary %v\n", proc.GetName(), os.Args)

	b, err := writer.JsonRecord(pid)
	if err != nil {
		log.Fatalf("FATAL %v marshal %v failed %v\n", pid, fn, err)
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
			_, err := fsl.SetFile(fn, b, np.NoOffset)
			if err != nil {
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			}
		}
	}

	pclnt.Exited(pid, proc.MakeStatus(proc.StatusOK))
}
