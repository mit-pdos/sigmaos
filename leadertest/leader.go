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
	sc, err := sigmaclnt.MkSigmaClnt("leader-" + pid.String())
	if err != nil {
		db.DFatalf("%v SigmaClnt %v failed %v\n", proc.GetName(), LEADERFN, err)
	}

	sc.Started()

	fn := dir + "/out"
	l := leaderclnt.MakeLeaderClnt(sc.FsLib, LEADERFN, 0777)

	epoch, err := l.AcquireFencedEpoch(nil, []string{dir})
	if err != nil {
		db.DFatalf("%v AcquireEpoch %v failed %v\n", proc.GetName(), LEADERFN, err)
	}

	log.Printf("%v: leader at %v\n", proc.GetName(), epoch)

	//
	// Write dir in new epoch
	//
	conf := &Config{epoch.String(), pid, pid}
	b, err := json.Marshal(*conf)
	if err != nil {
		db.DFatalf("%v marshal %v failed %v\n", proc.GetName(), fn, err)
	}
	_, err = sc.SetFile(fn, b, sp.OAPPEND, sp.NoOffset)
	if err != nil {
		db.DFatalf("%v SetFile b %v failed %v\n", proc.GetName(), fn, err)
	}

	if child == "child" {
		// Create a proc running in the same epoch as leader
		p := proc.MakeProc("leadertest-proc", []string{epoch.String(), dir})
		if err := sc.Spawn(p); err != nil {
			sc.Exited(proc.MakeStatusErr(err.Error(), nil))
			return
		}
		if err := sc.WaitStart(p.GetPid()); err != nil {
			sc.Exited(proc.MakeStatusErr(err.Error(), nil))
			return
		}
	}

	if last == "last" {
		// allow others to write for a while
		time.Sleep(500 * time.Millisecond)
	} else {
		if err := sc.Disconnect(sp.NAMED); err != nil {
			db.DFatalf("disconnect failed %v\n", err)
		}

		// wait a little before starting to write
		time.Sleep(2 * sp.Conf.Session.TIMEOUT)

		// these writes should fail since new leader will have started new epoch
		for i := 0; i < NWRITE; i++ {
			_, err := sc.SetFile(fn, b, sp.OAPPEND, sp.NoOffset)
			if err != nil {
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			}
		}
	}

	sc.Exited(proc.MakeStatus(proc.StatusOK))
}
