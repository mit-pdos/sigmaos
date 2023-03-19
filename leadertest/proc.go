package leadertest

import (
	"encoding/json"
	"log"
	"time"

	db "sigmaos/debug"
	"sigmaos/delay"
	"sigmaos/fenceclnt"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

func RunProc(epochstr, dir string) {
	pid := proc.GetPid()

	fsl, err := fslib.MakeFsLib("proc-" + pid.String())
	if err != nil {
		db.DFatalf("%v MakeFsLib err %v\n", proc.GetName(), err)
	}
	pclnt := procclnt.MakeProcClnt(fsl)
	pclnt.Started()

	epoch, err := sessp.String2Epoch(epochstr)
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
		_, err := fsl.SetFile(fn, b, sp.OAPPEND, sp.NoOffset)
		if err != nil {
			log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			break
		}
	}

	pclnt.ExitedOK()
}
