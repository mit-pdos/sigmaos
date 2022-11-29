package leadertest

import (
	"encoding/json"
	"log"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/leaderclnt"
	np "sigmaos/sigmap"
	"sigmaos/proc"
	"sigmaos/procclnt"
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
	fsl := fslib.MakeFsLib("leader-" + pid.String())
	pclnt := procclnt.MakeProcClnt(fsl)

	pclnt.Started()

	fn := dir + "/out"
	l := leaderclnt.MakeLeaderClnt(fsl, LEADERFN, 0777)

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
	_, err = fsl.SetFile(fn, b, np.OAPPEND, np.NoOffset)
	if err != nil {
		db.DFatalf("%v SetFile b %v failed %v\n", proc.GetName(), fn, err)
	}

	if child == "child" {
		// Create a proc running in the same epoch as leader
		p := proc.MakeProc("user/leadertest-proc", []string{epoch.String(), dir})
		if err := pclnt.Spawn(p); err != nil {
			pclnt.Exited(proc.MakeStatusErr(err.Error(), nil))
			return
		}
		if err := pclnt.WaitStart(p.Pid); err != nil {
			pclnt.Exited(proc.MakeStatusErr(err.Error(), nil))
			return
		}
	}

	if last == "last" {
		// allow others to write for a while
		time.Sleep(500 * time.Millisecond)
	} else {
		if err := fsl.Disconnect(np.NAMED); err != nil {
			db.DFatalf("disconnect failed %v\n", err)
		}

		// wait a little before starting to write
		time.Sleep(2 * np.Conf.Session.TIMEOUT)

		// these writes should fail since new leader will have started new epoch
		for i := 0; i < NWRITE; i++ {
			_, err := fsl.SetFile(fn, b, np.OAPPEND, np.NoOffset)
			if err != nil {
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			}
		}
	}

	pclnt.Exited(proc.MakeStatus(proc.StatusOK))
}
