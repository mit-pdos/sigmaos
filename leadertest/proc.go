package leadertest

import (
	"encoding/json"
	"time"

	db "sigmaos/debug"
	"sigmaos/fenceclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func RunProc(fencestr, dir string) {

	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("MkSigmaClnt err %v\n", err)
	}
	sc.Started()

	fence, err := sp.NewFenceJson([]byte(fencestr))
	if err != nil {
		sc.ClntExit(proc.MakeStatusErr(err.Error(), nil))
	}

	db.DPrintf(db.ALWAYS, "fence %v dir %v\n", fence, dir)

	fc := fenceclnt.MakeFenceClnt(sc.FsLib)
	if err := fc.FenceAtEpoch(*fence, []string{dir}); err != nil {
		sc.ClntExit(proc.MakeStatusErr(err.Error(), nil))
		return
	}

	fn := dir + "/out"

	conf := &Config{fence.Epoch, "", sc.ProcEnv().GetPID()}

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
			db.DPrintf(db.ALWAYS, "SetFile %v failed %v", fn, err)
			break
		}
	}

	sc.ClntExitOK()
}
