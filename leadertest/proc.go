package leadertest

import (
	"encoding/json"
	"log"
	"time"

	"sigmaos/delay"
	"sigmaos/fenceclnt"
	"sigmaos/fslib"
	np "sigmaos/sigmap"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

func RunProc(epochstr, dir string) {
	pid := proc.GetPid()

	fsl := fslib.MakeFsLib("proc-" + pid.String())
	pclnt := procclnt.MakeProcClnt(fsl)
	pclnt.Started()

	epoch, err := np.String2Epoch(epochstr)
	if err != nil {
		pclnt.Exited(proc.MakeStatusErr(err.Error(), nil))
	}

	fc := fenceclnt.MakeLeaderFenceClnt(fsl, LEADERFN)

	log.Printf("%v: epoch %v dir %v\n", proc.GetName(), epoch, dir)

	if err := fc.FenceAtEpoch(epoch, []string{dir}); err != nil {
		pclnt.Exited(proc.MakeStatusErr(err.Error(), nil))
		return
	}

	fn := dir + "/out"

	conf := &Config{epochstr, "", pid}

	// wait a little before starting to write
	time.Sleep(10 * time.Millisecond)

	// and delay writes
	delay.Delay(DELAY)

	b, err := json.Marshal(*conf)
	if err != nil {
		pclnt.Exited(proc.MakeStatusErr(err.Error(), nil))
		return
	}

	for i := 0; i < NWRITE; i++ {
		_, err := fsl.SetFile(fn, b, np.OAPPEND, np.NoOffset)
		if err != nil {
			log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			break
		}
	}

	pclnt.Exited(proc.MakeStatus(proc.StatusOK))
}
