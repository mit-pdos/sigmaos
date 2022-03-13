package leadertest

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

func RunSecondary(fence, dir string) {
	pid := proc.GetPid()
	fsl := fslib.MakeFsLib("primary-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)

	pclnt.Started(pid)

	fn := dir + "/out"
	f := fenceclnt.MakeFenceClnt(fsl, os.Args[1], 0, []string{dir})
	log.Printf("%v: secondary %v\n", proc.GetName(), os.Args)
	for {
		config := &Config{}
		err := f.AcquireConfig(config)
		if err != nil {
			log.Fatalf("FATAL %v Config %v failed %v\n", pid, os.Args[1], err)
		}

		// wait a little before starting to write
		time.Sleep(10 * time.Millisecond)

		// and delay writes
		delay.Delay(DELAY)

		b, err := writer.JsonRecord(pid)
		if err != nil {
			log.Fatalf("FATAL %v marshal %v failed %v\n", pid, fn, err)
		}

		for i := 0; i < N; i++ {
			_, err := fsl.SetFile(fn, b, np.NoOffset)
			if err != nil {
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
				break
			}
		}
	}
	pclnt.Exited(pid, proc.MakeStatus(proc.StatusOK))
}
