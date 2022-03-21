package leadertest

import (
	"log"
	"time"

	"ulambda/delay"
	"ulambda/fslib"
	"ulambda/leaderclnt"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/writer"
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

	pclnt.Started(pid)

	fn := dir + "/out"
	l := leaderclnt.MakeLeaderClnt(fsl, LEADERFN, 0777)

	epoch, err := l.AcquireFencedEpoch(nil, []string{dir})
	if err != nil {
		log.Fatalf("FATAL %v AcquireEpoch %v failed %v\n", proc.GetName(), LEADERFN, err)
	}

	log.Printf("%v: leader at %v\n", proc.GetName(), epoch)

	//
	// Write dir in new epoch
	//
	conf := &Config{epoch.String(), pid, pid}
	b, err := writer.JsonRecord(*conf)
	if err != nil {
		log.Fatalf("FATAL %v marshal %v failed %v\n", proc.GetName(), fn, err)
	}
	_, err = fsl.SetFile(fn, b, np.OAPPEND, np.NoOffset)
	if err != nil {
		log.Fatalf("FATAL %v SetFile b %v failed %v\n", proc.GetName(), fn, err)
	}

	if child == "child" {
		// Create a proc running in the same epoch as leader
		p := proc.MakeProc("bin/user/leadertest-proc", []string{epoch.String(), dir})
		if err := pclnt.Spawn(p); err != nil {
			pclnt.Exited(pid, proc.MakeStatusErr(err.Error(), nil))
			return
		}
		if err := pclnt.WaitStart(p.Pid); err != nil {
			pclnt.Exited(pid, proc.MakeStatusErr(err.Error(), nil))
			return
		}
	}

	if last == "last" {
		// allow others to write for a while
		time.Sleep(500 * time.Millisecond)
	} else {
		fsl.Disconnect(LEADERFN)

		// wait a little before starting to write
		time.Sleep(10 * time.Millisecond)

		// and delay writes
		delay.Delay(DELAY)

		// these writes should fail since new leader will have started new epoch
		for i := 0; i < NWRITE; i++ {
			_, err := fsl.SetFile(fn, b, np.OAPPEND, np.NoOffset)
			if err != nil {
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			}
		}
	}

	pclnt.Exited(pid, proc.MakeStatus(proc.StatusOK))
}
