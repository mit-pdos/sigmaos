package leadertest

import (
	"log"
	"strconv"
	"time"

	"ulambda/delay"
	"ulambda/epochclnt"
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
	EPOCH    = LEADERFN + "-epoch"
)

func RunLeader(dir, last, child string) {
	pid := proc.GetPid()
	fsl := fslib.MakeFsLib("leader-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)

	pclnt.Started(pid)

	fn := dir + "/out"
	l := leaderclnt.MakeLeaderClnt(fsl, LEADERFN, 0)
	ec := epochclnt.MakeEpochClnt(fsl, EPOCH, 0777, []string{dir})

	err := l.AcquireLeadership()
	if err != nil {
		log.Fatalf("FATAL %v AcquireLeader %v failed %v\n", pid, LEADERFN, err)
	}

	//
	// Start new Epoch
	//

	conf := &Config{}
	err = fsl.GetFileJson(EPOCH, conf)
	if err != nil && !np.IsErrNotfound(err) {
		log.Fatalf("FATAL %v PutFileAtomic %v failed %v\n", pid, CONFIGBK, err)
	}
	conf.N += 1
	conf.Leader = pid
	conf.Pid = pid

	err = ec.MakeEpochFileJson(*conf)
	if err != nil {
		log.Fatalf("FATAL %v MakeEpochFileFrom %v failed %v\n", pid, ec.Name(), err)
	}

	log.Printf("%v: leaderfn %v conf %v\n", proc.GetName(), LEADERFN, conf)

	//
	// Write in new epoch
	//

	b, err := writer.JsonRecord(*conf)
	if err != nil {
		log.Fatalf("FATAL %v marshal %v failed %v\n", proc.GetName(), fn, err)
	}
	_, err = fsl.SetFile(fn, b, np.NoOffset)
	if err != nil {
		log.Fatalf("FATAL %v SetFile b %v failed %v\n", proc.GetName(), fn, err)
	}

	if child == "child" {
		// Create a proc running in the same epoch as leader
		p := proc.MakeProc("bin/user/leadertest-proc",
			[]string{EPOCH, dir, strconv.Itoa(conf.N)})
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
			_, err := fsl.SetFile(fn, b, np.NoOffset)
			if err != nil {
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			}
		}
	}

	pclnt.Exited(pid, proc.MakeStatus(proc.StatusOK))
}
