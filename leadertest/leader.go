package leadertest

import (
	"log"
	"time"

	"ulambda/atomic"
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
	N        = 10
	DELAY    = 10
	LEADERFN = "name/leader"
	CONFIG   = LEADERFN + "-config"
	CONFIGBK = LEADERFN + "-config#"
	EPOCH    = LEADERFN + "-epoch"
)

func RunLeader(dir, last string) {
	pid := proc.GetPid()
	fsl := fslib.MakeFsLib("leader-" + proc.GetPid())
	pclnt := procclnt.MakeProcClnt(fsl)

	pclnt.Started(pid)

	fn := dir + "/out"
	l := leaderclnt.MakeLeaderClnt(fsl, LEADERFN, 0)

	err := l.AcquireLeadership()
	if err != nil {
		log.Fatalf("FATAL %v AcquireLeader %v failed %v\n", pid, LEADERFN, err)
	}

	log.Printf("%v: leader %v %v %v\n", proc.GetName(), LEADERFN, dir, last)

	// Start new Epoch

	conf := &Config{0, pid}
	ec := epochclnt.MakeEpochClnt(fsl, EPOCH, 0777, []string{dir})
	err = atomic.PutFileJsonAtomic(fsl, CONFIGBK, 0777, *conf)
	if err != nil {
		log.Fatalf("FATAL %v PutFileAtomic %v failed %v\n", pid, CONFIGBK, err)
	}
	err = ec.MakeEpochFileFrom(CONFIGBK)
	if err != nil {
		log.Fatalf("FATAL %v MakeEpochFileFrom %v failed %v\n", pid, ec.Name(), err)
	}

	// Write in new epoch

	b, err := writer.JsonRecord(pid)
	if err != nil {
		log.Fatalf("FATAL %v marshal %v failed %v\n", pid, fn, err)
	}
	_, err = fsl.SetFile(fn, b, np.NoOffset)
	if err != nil {
		log.Fatalf("FATAL %v SetFile b %v failed %v\n", pid, fn, err)
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
		for i := 0; i < N; i++ {
			_, err := fsl.SetFile(fn, b, np.NoOffset)
			if err != nil {
				log.Printf("%v: SetFile %v failed %v\n", proc.GetName(), fn, err)
			}
		}
	}

	pclnt.Exited(pid, proc.MakeStatus(proc.StatusOK))
}
