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
	sp "sigmaos/sigmap"
	"sigmaos/util/rand"
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

func unmarshalTevents(s string, evs *[]Tevent) error {
	if s == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), evs); err != nil {
		return err
	}
	return nil
}

func parseTevents(s string, labels map[Tselector]Tevent) error {
	var evs []Tevent
	if err := unmarshalTevents(s, &evs); err != nil {
		return err
	}
	for _, e := range evs {
		labels[Tselector(e.Label)] = e
	}
	return nil
}

func initLabels() {
	if labels != nil {
		return
	}
	labelstr := proc.GetSigmaFail()
	labels = make(map[Tselector]Tevent, len(labelstr))
	if err := parseTevents(labelstr, labels); err != nil {
		db.DFatalf("parseLabels %v err %v", labelstr, err)
	}
	db.DPrintf(db.CRASH, "Events %v", labels)
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

func SetSigmaFail(es []Tevent) error {
	s, err := MakeTevents(es)
	if err != nil {
		return err
	}
	proc.SetSigmaFail(s)
	return nil
}

func AppendSigmaFail(es []Tevent) error {
	var evs []Tevent
	s := proc.GetSigmaFail()
	if err := unmarshalTevents(s, &evs); err != nil {
		return err
	}
	evs = append(evs, es...)
	return SetSigmaFail(evs)
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
	initLabels()
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
