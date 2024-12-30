package leadertest

import (
	"encoding/json"
	"path/filepath"
	"time"

	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	NWRITE   = 10
	DELAY    = 10
	LEADERFN = "name/leader"
	CONFIG   = LEADERFN + "-config"
	CONFIGBK = LEADERFN + "-config#"
	OUT      = "out"
)

func RunLeader(dir, last, child string) {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("SigmaClnt %v failed %v\n", LEADERFN, err)
	}

	sc.Started()

	fn := filepath.Join(dir, OUT)
	l, err := leaderclnt.NewLeaderClnt(sc.FsLib, LEADERFN, 0777)
	if err != nil {
		db.DFatalf("NewLeaderClnt %v failed %v\n", LEADERFN, err)
	}

	if err := l.LeadAndFence(nil, []string{dir}); err != nil {
		db.DFatalf("AcquireEpoch %v failed %v\n", LEADERFN, err)
	}

	db.DPrintf(db.ALWAYS, "leader at %v\n", l.Fence())

	//
	// Write dir in new epoch
	//
	conf := &Config{l.Fence().Epoch, sc.ProcEnv().GetPID(), sc.ProcEnv().GetPID()}
	b, err := json.Marshal(*conf)
	if err != nil {
		db.DFatalf("marshal %v failed %v\n", fn, err)
	}
	_, err = sc.SetFile(fn, b, sp.OAPPEND, sp.NoOffset)
	if err != nil {
		db.DFatalf("SetFile b %v failed %v\n", fn, err)
	}

	if child == "child" {
		// Create a proc running in the same fence as leader
		b := l.Fence().Json()
		p := proc.NewProc("leadertest-proc", []string{string(b), dir})
		if err := sc.Spawn(p); err != nil {
			sc.ClntExit(proc.NewStatusErr(err.Error(), nil))
			return
		}
		if err := sc.WaitStart(p.GetPid()); err != nil {
			sc.ClntExit(proc.NewStatusErr(err.Error(), nil))
			return
		}
	}

	if last == "last" {
		// allow others to write for a while
		time.Sleep(500 * time.Millisecond)
	} else {

		l.ReleaseLeadership()

		// wait a little before starting to write
		time.Sleep(1 * time.Second)

		// these writes should fail since new leader will have started new epoch
		for i := 0; i < NWRITE; i++ {
			_, err := sc.SetFile(fn, b, sp.OAPPEND, sp.NoOffset)
			if err != nil {
				db.DPrintf(db.ALWAYS, "SetFile %v failed %v\n", fn, err)
			}
		}
	}

	sc.ClntExitOK()
}
