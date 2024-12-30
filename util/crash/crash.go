// The crash package is used by procs to randomly crash and
// introduce permanant/temporary network partitions.
package crash

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/coordination/semaphore"
	"sigmaos/util/rand"
)

const (
	ONE   = 1000
	FIFTY = 500
)

var labels *TeventMap

type Tevent struct {
	Label string `json:"label"` // see selector.go

	// wait for start ms to start generating events
	Start int64 `json:"start"`

	// max length of event interval in ms (if <= 0, only once)
	MaxInterval int64 `json:"maxinterval"`

	// probability of generating event in this interval
	Prob float64 `json:"prob:`

	// delay in ms (interpretable by event creator)
	Delay int64 `json:"delay"`

	// pathname for, for example, a semaphore to delay event
	// generation until semaphore has been upped.
	Path string
}

func NewEvent(l string, mi int64, p float64) Tevent {
	return Tevent{Label: l, MaxInterval: mi, Prob: p}
}

func NewEventPath(l string, mi int64, p float64, pn string) Tevent {
	return Tevent{Label: l, MaxInterval: mi, Prob: p, Path: pn}
}

func NewEventStart(l string, s, mi int64, p float64) Tevent {
	return Tevent{Label: l, Start: s, MaxInterval: mi, Prob: p}
}

func NewEventStartDelay(l string, s, mi int64, d int64, p float64) Tevent {
	return Tevent{Label: l, Start: s, MaxInterval: mi, Delay: d, Prob: p}
}

func (e *Tevent) String() string {
	return fmt.Sprintf("{l %v s %v mi %v p %v d %v}", e.Label, e.Start, e.MaxInterval, e.Prob, e.Delay)
}

type Teventf func(e Tevent)

type TeventMap struct {
	Evs map[Tselector]Tevent `json:"evs"`
}

func NewTeventMap() *TeventMap {
	return &TeventMap{Evs: make(map[Tselector]Tevent)}
}

func NewTeventMapOne(e Tevent) *TeventMap {
	em := NewTeventMap()
	em.Evs[Tselector(e.Label)] = e
	return em
}

func (em *TeventMap) Events2String() (string, error) {
	b, err := json.Marshal(em)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (em *TeventMap) Insert(e Tevent) {
	em.Evs[Tselector(e.Label)] = e
}

func (em *TeventMap) Lookup(l Tselector) (Tevent, bool) {
	e, ok := em.Evs[l]
	return e, ok
}

func (em *TeventMap) Merge(em0 *TeventMap) {
	for k, v := range em0.Evs {
		em.Evs[k] = v
	}
}

func (em0 *TeventMap) Filter(l Tselector) *TeventMap {
	em1 := NewTeventMap()
	for k, v := range em0.Evs {
		if k == l {
			em1.Evs[k] = v
		}
	}
	return em1
}

func (em *TeventMap) AppendEnv(p *proc.Proc) error {
	s, err := em.Events2String()
	if err != nil {
		return err
	}
	p.AppendEnv(proc.SIGMAFAIL, s)
	return nil
}

func unmarshalTevents(s string) (*TeventMap, error) {
	if s == "" {
		return NewTeventMap(), nil
	}
	em := NewTeventMap()
	if err := json.Unmarshal([]byte(s), em); err != nil {
		return nil, err
	}
	return em, nil
}

func initLabels() {
	if labels != nil {
		return
	}
	labelstr := proc.GetSigmaFail()
	em, err := unmarshalTevents(labelstr)
	if err != nil {
		db.DFatalf("parseLabels %v err %v", labelstr, err)
	}
	labels = em
	db.DPrintf(db.CRASH, "Events %v", labels)
}

func Rand50() bool {
	return rand.Int64(ONE) < FIFTY
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

func SetSigmaFail(em *TeventMap) error {
	s, err := em.Events2String()
	if err != nil {
		return err
	}
	proc.SetSigmaFail(s)
	return nil
}

func AppendSigmaFail(em1 *TeventMap) error {
	s := proc.GetSigmaFail()
	em0, err := unmarshalTevents(s)
	if err != nil {
		return err
	}
	em0.Merge(em1)
	return SetSigmaFail(em0)
}

func Crash() {
	db.DPrintf(db.CRASH, "Crash")
	os.Exit(proc.CRASH)
}

func CrashMsg(msg string) {
	db.DPrintf(db.CRASH, "CrashMsg %v", msg)
	os.Exit(proc.CRASH)
}

func PartitionNamed(fsl *fslib.FsLib) {
	db.DPrintf(db.CRASH, "PartitionNamed from %v", sp.NAMED)
	if err := fsl.Disconnect(sp.NAMED); err != nil {
		db.DPrintf(db.CRASH, "Disconnect %v name fails err %v", os.Args, err)
	}
}

func PartitionAll(fsl *fslib.FsLib) {
	db.DPrintf(db.CRASH, "PartitionAll")
	if err := fsl.Disconnect(""); err != nil {
		db.DPrintf(db.CRASH, "Disconnect %v name fails err %v", os.Args, err)
	}
}

func PartitionPath(fsl *fslib.FsLib, pn string) {
	db.DPrintf(db.CRASH, "PartitionPath %v", pn)
	if _, err := fsl.Stat(pn); err != nil {
		db.DPrintf(db.CRASH, "PartitionPath: %v Stat %v err %v", os.Args, pn, err)
	}
	if err := fsl.Disconnect(pn); err != nil {
		db.DPrintf(db.CRASH, "PartitionPath: %v Disconnect %v err %v", os.Args, pn, err)
	}
}

func Failer(fsl *fslib.FsLib, label Tselector, f Teventf) {
	initLabels()
	if e, ok := labels.Evs[label]; ok {
		go func(label Tselector, e Tevent) {
			if e.Path != "" {
				sem := semaphore.NewSemaphore(fsl, e.Path)
				sem.Init(0)
				sem.Down()
				db.DPrintf(db.CRASH, "Downed %v", e.Path)
			}
			time.Sleep(time.Duration(e.Start) * time.Millisecond)
			for true {
				t := e.MaxInterval
				if e.MaxInterval < 0 {
					t = -t
				}
				r := randSleep(t)
				if r < uint64(e.Prob*ONE) {
					db.DPrintf(db.CRASH, "Raise event %v r %d %v", label, r, e)
					f(e)
				}
				if e.MaxInterval <= 0 {
					break
				}
			}
		}(label, e)
	}
}

func FailersDefault(fsl *fslib.FsLib, labels []Tselector) {
	defaults := []Teventf{
		func(e Tevent) {
			Crash()
		},
		func(e Tevent) {
			PartitionNamed(fsl)
		},
	}
	for i, l := range labels {
		Failer(fsl, l, defaults[i])
	}
}

func SignalFailer(fsl *fslib.FsLib, fn string) error {
	db.DPrintf(db.CRASH, "Signal %v", fn)
	sem := semaphore.NewSemaphore(fsl, fn)
	return sem.Up()
}
