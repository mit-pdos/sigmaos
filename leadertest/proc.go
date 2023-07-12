package leadertest

import (
	"encoding/json"
	"log"
	"time"

	db "sigmaos/debug"
	"sigmaos/delay"
	"sigmaos/fenceclnt"
	"sigmaos/proc"
	"sigmaos/sessp"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func RunProc(epochstr, dir string) {
	pid := proc.GetPid()

	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname("proc-" + pid.String()))
	if err != nil {
		db.DFatalf("%v MkSigmaClnt err %v\n", proc.GetName(), err)
	}
	sc.Started()

	epoch, err := sessp.String2Epoch(epochstr)
	if err != nil {
		sc.Exit(proc.MakeStatusErr(err.Error(), nil))
	}

	fc := fenceclnt.MakeLeaderFenceClnt(sc.FsLib, LEADERFN)

	log.Printf("%v: epoch %v dir %v\n", proc.GetName(), epoch, dir)

	if err := fc.FenceAtEpoch(epoch, []string{dir}); err != nil {
		sc.Exit(proc.MakeStatusErr(err.Error(), nil))
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
		sc.Exit(proc.MakeStatusErr(err.Error(), nil))
		return
	}

	for i := 0; i < NWRITE; i++ {
		_, err := sc.SetFile(fn, b, sp.OAPPEND, sp.NoOffset)
		if err != nil {
			log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			break
		}
	}

	sc.ExitOK()
}
