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

func RunProc(epochfn, epochstr, dir string) {
	pid := proc.GetPid()

	fsl := fslib.MakeFsLib("primary-" + pid)
	pclnt := procclnt.MakeProcClnt(fsl)

	epoch, err := np.String2Epoch(epochstr)
	if err != nil {
		pclnt.Exited(pid, proc.MakeStatusErr(err.Error(), nil))
	}

	fc := fenceclnt1.MakeEpochFenceClnt(fsl, epochfn, 0, []string{dir})

	pclnt.Started(pid)

	fn := dir + "/out"

	log.Printf("%v: epochfn %v epoch %v dir %v\n", proc.GetName(), epochfn, epoch, dir)

	if err := fc.FenceAtEpoch(epoch); err != nil {
		pclnt.Exited(pid, proc.MakeStatusErr(err.Error(), nil))
		return
	}

	conf := &Config{epochstr, "", pid}

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
