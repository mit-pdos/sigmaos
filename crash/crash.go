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
	sp "sigmaos/sigmap"
)

const ONE = 1000

var labels map[Tselector]Tevent

type Tevent struct {
	Label string `json:"label"` // see selector.go

	// wait for start ms to start generating events
	Start int64 `json:"start"`

	// max length of event interval in ms (if 0, only once)
	MaxInterval int64 `json:"maxinterval"`

	// probability of generating event in this interval
	Prob float64 `json:"prob:`

	// delay in ms (interpretable by event creator)
	Delay int64 `json:"delay"`
}

type Teventf func(e Tevent)

func (e *Tevent) String() string {
	return fmt.Sprintf("{l %v s %v mi %v p %v d %v}", e.Label, e.Start, e.MaxInterval, e.Prob, e.Delay)
}

func MakeTevents(es []Tevent) (string, error) {
	b, err := json.Marshal(es)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseTevents(s string, labels map[Tselector]Tevent) error {
	var evs []Tevent
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
	labels = make(map[Tselector]Tevent, len(labelstr))
	if err := parseTevents(labelstr, labels); err != nil {
		db.DFatalf("parseLabels %v err %v", labelstr, err)
	}
	// db.DPrintf(db.CRASH, "Events %v", labels)
	db.DPrintf(db.SPAWN_LAT, "[%v] crash init pkg: %v", proc.GetSigmaDebugPid(), time.Since(s))
}

func randSleep(c int64) uint64 {
	ms := uint64(0)
	if c > 0 {
		ms = rand.Int64(c)
	}
	r := rand.Int64(ONE)
	// db.DPrintf(db.CRASH, "randSleep %dms r %d\n", ms, r)
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

// New interface

func AppendSigmaFail(es []Tevent) error {
	s, err := MakeTevents(es)
	if err != nil {
		return err
	}
	proc.AppendSigmaFail(s)
	return nil
}

func Crash() {
	db.DPrintf(db.CRASH, "Crash: Exit")
	os.Exit(1)
}

func CrashMsg(msg string) {
	db.DPrintf(db.CRASH, "CrashMsg %v", msg)
	os.Exit(1)
}

func PartitionNamed(fsl *fslib.FsLib) {
	db.DPrintf(db.CRASH, "PartitionNamed from %v\n", sp.NAMED)
	if error := fsl.Disconnect(sp.NAMED); error != nil {
		db.DPrintf(db.CRASH, "Disconnect %v name fails err %v\n", os.Args, error)
	}
}

func Failer(label Tselector, f Teventf) {
	if e, ok := labels[label]; ok {
		go func(label Tselector, e Tevent) {
			time.Sleep(time.Duration(e.Start) * time.Millisecond)
			for true {
				r := randSleep(e.MaxInterval)
				if r < uint64(e.Prob*ONE) {
					db.DPrintf(db.CRASH, "Label %v r %d %v", label, r, e)
					f(e)
				}
				if e.MaxInterval == 0 {
					break
				}
			}
		}(label, e)
	} else {
		db.DPrintf(db.CRASH, "Unknown label %v", label)
	}
}

func FailersDefault(labels []Tselector, fsl *fslib.FsLib) {
	defaults := []Teventf{
		func(e Tevent) {
			Crash()
		},
		func(e Tevent) {
			PartitionNamed(fsl)
		},
	}
	for i, l := range labels {
		Failer(l, defaults[i])
	}
}
