package leadertest

import (
	"log"
	"time"

	"ulambda/delay"
	"ulambda/fenceclnt1"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/writer"
)

func RunProc(epochfn, epoch, dir string) {
	pid := proc.GetPid()
	fsl := fslib.MakeFsLib("primary-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)
	fc := fenceclnt1.MakeFenceClnt(fsl, epochfn, 0, []string{dir})

	pclnt.Started(pid)

	fn := dir + "/out"

	log.Printf("%v: epochfn %v epoch %v dir %v\n", proc.GetName(), epochfn, epoch, dir)

	err := fc.FenceEpochAt(epoch)
	if err != nil {
		pclnt.Exited(pid, proc.MakeStatusErr(err.Error(), nil))
		return
	}

	conf := &Config{epoch, "", pid}

	// wait a little before starting to write
	time.Sleep(10 * time.Millisecond)

	// and delay writes
	delay.Delay(DELAY)

	b, err := writer.JsonRecord(*conf)
	if err != nil {
		pclnt.Exited(pid, proc.MakeStatusErr(err.Error(), nil))
		return
	}

	for i := 0; i < NWRITE; i++ {
		_, err := fsl.SetFile(fn, b, np.NoOffset)
		if err != nil {
			log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			break
		}
	}

	pclnt.Exited(pid, proc.MakeStatus(proc.StatusOK))
}
