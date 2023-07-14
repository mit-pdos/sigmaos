package leadertest

import (
	"encoding/json"
	"log"
	"time"

	db "sigmaos/debug"
	"sigmaos/leaderclnt"
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
)

func RunLeader(dir, last, child string) {
	pid := proc.GetPid()
	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname("leader-" + pid.String()))
	if err != nil {
		db.DFatalf("%v SigmaClnt %v failed %v\n", proc.GetName(), LEADERFN, err)
	}

	sc.Started()

	fn := dir + "/out"
	l, err := leaderclnt.MakeLeaderClnt(sc.FsLib, LEADERFN, 0777)
	if err != nil {
		db.DFatalf("%v MakeLeaderClnt %v failed %v\n", proc.GetName(), LEADERFN, err)
	}

	if err := l.LeadAndFence(nil, []string{dir}); err != nil {
		db.DFatalf("%v AcquireEpoch %v failed %v\n", proc.GetName(), LEADERFN, err)
	}

	log.Printf("%v: leader at %v\n", proc.GetName(), l.Fence())

	//
	// Write dir in new epoch
	//
	conf := &Config{l.Fence().Epoch, pid, pid}
	b, err := json.Marshal(*conf)
	if err != nil {
		db.DFatalf("%v marshal %v failed %v\n", proc.GetName(), fn, err)
	}
	_, err = sc.SetFile(fn, b, sp.OAPPEND, sp.NoOffset)
	if err != nil {
		db.DFatalf("%v SetFile b %v failed %v\n", proc.GetName(), fn, err)
	}

	if child == "child" {
		// Create a proc running in the same fence as leader
		f := l.Fence()
		b, err := json.Marshal(*f)
		if err != nil {
			db.DFatalf("%v marshal err %v\n", proc.GetName(), err)
		}
		p := proc.MakeProc("leadertest-proc", []string{string(b), dir})
		if err := sc.Spawn(p); err != nil {
			sc.Exit(proc.MakeStatusErr(err.Error(), nil))
			return
		}
		if err := sc.WaitStart(p.GetPid()); err != nil {
			sc.Exit(proc.MakeStatusErr(err.Error(), nil))
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
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			}
		}
	}

	sc.ExitOK()
}
