// The crash package is used by procs to randomly crash and
// introduce permanant/temporary network partitions.
package crash

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const ONE = 1000

var labels map[Tselector]Event

type Event struct {
	Label       string  `json:"label"`       // see selector.go
	Start       int64   `json:"start"`       // wait for start ms to start generating events
	MaxInterval int64   `json:"maxinterval"` // max length of event interval in ms
	Prob        float64 `json:"prob:`        // probability of generating event in this interval
	Delay       int64   `json:"delay"`       // delay in ms (interpretable by event creator)
}

func (e *Event) String() string {
	return fmt.Sprintf("{l %v s %v mi %v p %v d %v}", e.Label, e.Start, e.MaxInterval, e.Prob, e.Delay)
}

func MakeEvents(es []Event) (string, error) {
	b, err := json.Marshal(es)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseEvents(s string, labels map[Tselector]Event) error {
	var evs []Event
	if s == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), &evs); err != nil {
		return err
	}
	for _, e := range evs {
		labels[Tselector(e.Label)] = e
	}
	return nil
}

func init() {
	s := time.Now()
	labelstr := proc.GetSigmaFail()
	labels = make(map[Tselector]Event, len(labelstr))
	if err := parseEvents(labelstr, labels); err != nil {
		db.DFatalf("parseLabels %v err %v", labelstr, err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] crash init pkg: %v", proc.GetSigmaDebugPid(), time.Since(s))
}

func randSleep(c int64) uint64 {
	ms := rand.Int64(c)
	r := rand.Int64(ONE)
	db.DPrintf(db.CRASH, "randSleep %dms r %d\n", ms, r)
	time.Sleep(time.Duration(ms) * time.Millisecond)
	return r
}

func Crasher(fsl *fslib.FsLib) {
	crash := fsl.ProcEnv().GetCrash()
	if crash == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(crash)
			if r < 330 {
				fail(crash, nil)
			} else if r < 660 {
				PartitionNamed(fsl)
			}
		}
	}()
}

func CrasherMsg(fsl *fslib.FsLib, f func() string) {
	crash := fsl.ProcEnv().GetCrash()
	if crash == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(crash)
			if r < 330 {
				fail(crash, f)
			} else if r < 660 {
				PartitionNamed(fsl)
			}
		}
	}()
}

func Partitioner(ss *sesssrv.SessSrv) {
	part := ss.ProcEnv().GetPartition()
	if part == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(part)
			if r < 330 {
				ss.PartitionClient(true)
			}
		}
	}()
}

func NetFailer(ss *sesssrv.SessSrv) {
	crash := ss.ProcEnv().GetNetFail()
	if crash == 0 {
		return
	}
	go func() {
		for true {
			r := randSleep(crash)
			if r < 330 {
				ss.PartitionClient(false)
			}
		}
	}()
}

// Randomly tell parent we exited but then keep running, simulating a
// network partition from the parent's point of view.
func PartitionParentProb(sc *sigmaclnt.SigmaClnt, prob uint64) bool {
	crash := sc.ProcEnv().GetCrash()
	if crash == 0 {
		return false
	}
	p := rand.Int64(100)
	if p < prob {
		db.DPrintf(db.CRASH, "PartitionParentProb %v p %v\n", prob, p)
		sc.ProcAPI.Exited(proc.NewStatusErr("partitioned", nil))
		return true
	}
	return false
}

func fail(crash int64, f func() string) {
	msg := ""
	if f != nil {
		msg = f()
	}
	db.DPrintf(db.CRASH, "crash.fail %v %v\n", crash, msg)
	os.Exit(1)
}

func Fail(crash int64) {
	fail(crash, nil)
}

func Crash() {
	db.DPrintf(db.CRASH, "Crash")
	os.Exit(1)
}

func PartitionNamed(fsl *fslib.FsLib) {
	db.DPrintf(db.CRASH, "PartitionNamed from %v\n", sp.NAMED)
	if error := fsl.Disconnect(sp.NAMED); error != nil {
		db.DPrintf(db.CRASH, "Disconnect %v name fails err %v\n", os.Args, error)
	}
}

func Failer(label Tselector, f func(e Event)) {
	if e, ok := labels[label]; ok {
		go func() {
			time.Sleep(time.Duration(e.Start) * time.Millisecond)
			for true {
				r := randSleep(e.MaxInterval)
				if r < uint64(e.Prob*ONE) {
					f(e)
				}
			}
		}()
	} else {
		db.DPrintf(db.TEST, "Unknown label %v", label)
	}
}
