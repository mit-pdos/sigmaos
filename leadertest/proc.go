package leadertest

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"ulambda/delay"
	"ulambda/epochclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/writer"
)

func RunProc(epoch, dir, N string) {
	pid := proc.GetPid()
	fsl := fslib.MakeFsLib("primary-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)

	pclnt.Started(pid)

	n, err := strconv.Atoi(N)
	if err != nil {
		pclnt.Exited(pid, proc.MakeStatusErr(err.Error(), nil))
		return
	}

	fn := dir + "/out"
	ec := epochclnt.MakeEpochClnt(fsl, epoch, 0, []string{dir})
	log.Printf("%v: epochfn %v dir %v N %v\n", proc.GetName(), epoch, dir, N)

	conf := &Config{}
	if err := ec.GetFileJson(EPOCH, conf); err != nil {
		pclnt.Exited(pid, proc.MakeStatusErr(err.Error(), nil))
		return
	}
	if n != conf.N {
		pclnt.Exited(pid, proc.MakeStatusErr(fmt.Sprintf("wrong epoch %v %v", n, conf.N), nil))
		return

	}

	// Fence writes in this epoch
	if err := ec.FenceOffEpoch(); err != nil {
		pclnt.Exited(pid, proc.MakeStatusErr(fmt.Sprintf("wrong epoch %v %v", n, conf.N), nil))
		return
	}

	conf.Pid = pid

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
