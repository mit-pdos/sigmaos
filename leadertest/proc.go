package leadertest

import (
	"encoding/json"
	"log"
	"time"

	db "sigmaos/debug"
	"sigmaos/fenceclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func RunProc(fencestr, dir string) {
	pid := proc.GetPid()

	sc, err := sigmaclnt.NewSigmaClnt(sp.Tuname("proc-" + pid.String()))
	if err != nil {
		db.DFatalf("%v MkSigmaClnt err %v\n", proc.GetName(), err)
	}
	sc.Started()

	fence, err := sp.NewFenceJson([]byte(fencestr))
	if err != nil {
		sc.ClntExit(proc.MakeStatusErr(err.Error(), nil))
	}

	log.Printf("%v: fence %v dir %v\n", proc.GetName(), fence, dir)

	fc := fenceclnt.MakeFenceClnt(sc.FsLib)
	if err := fc.FenceAtEpoch(*fence, []string{dir}); err != nil {
		sc.ClntExit(proc.MakeStatusErr(err.Error(), nil))
		return
	}

	fn := dir + "/out"

	conf := &Config{fence.Epoch, "", pid}

	// wait a little before starting to write
	time.Sleep(10 * time.Millisecond)

	b, err := json.Marshal(*conf)
	if err != nil {
		sc.ClntExit(proc.MakeStatusErr(err.Error(), nil))
		return
	}

	for i := 0; i < NWRITE; i++ {
		_, err := sc.SetFile(fn, b, sp.OAPPEND, sp.NoOffset)
		if err != nil {
			log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			break
		}
	}

	sc.ClntExitOK()
}
