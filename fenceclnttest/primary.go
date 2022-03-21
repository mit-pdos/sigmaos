package fenceclnttest

import (
	"log"
	"time"

	"ulambda/atomic"
	"ulambda/delay"
	"ulambda/fenceclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/writer"
)

const (
	N      = 10
	DELAY  = 10
	CONFIG = "-config"
)

func conffn(fn string) string {
	return fn + "CONFIG"
}

func RunPrimary(fence, dir, last string) {
	pid := proc.GetPid()
	fsl := fslib.MakeFsLib("primary-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)

	pclnt.Started(pid)

	fn := dir + "/out"
	f := fenceclnt.MakeFenceClnt(fsl, fence, 0, []string{dir})

	err := f.AcquireFenceW([]byte(pid))
	if err != nil {
		log.Fatalf("FATAL %v AcquireFenceW %v failed %v\n", pid, fence, err)
	}

	log.Printf("%v: primary %v %v %v\n", proc.GetName(), fence, dir, last)

	b, err := writer.JsonRecord(pid)
	if err != nil {
		log.Fatalf("FATAL %v marshal %v failed %v\n", pid, fn, err)
	}
	_, err = fsl.SetFile(fn, b, np.NoOffset)
	if err != nil {
		log.Fatalf("FATAL %v SetFile b %v failed %v\n", pid, fn, err)
	}

	conf := &Config{0, pid}
	err = atomic.PutFileJsonAtomic(fsl, conffn(fence), 0777, conf)
	if err != nil {
		log.Fatalf("FATAL %v: MakeFile %v err %v\n", proc.GetName(), conffn(fence), err)
	}

	if last == "last" {
		// allow others to write for a while
		time.Sleep(500 * time.Millisecond)
	} else {
		fsl.Disconnect(fence)

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
