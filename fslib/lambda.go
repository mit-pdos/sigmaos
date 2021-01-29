package fslib

import (
	"encoding/json"
	"math/rand"
	"strconv"

	np "ulambda/ninep"
)

type PDep struct {
	Producer string
	Consumer string
}

type Attr struct {
	Pid     string
	Program string
	Args    []string
	Env     []string
	PairDep []PDep
	ExitDep []string
}

const (
	SCHED    = "name/schedd"
	SCHEDDEV = SCHED + "/" + "dev"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}

func (fl *FsLib) Spawn(a *Attr) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	b = append([]byte("Spawn "), b...)
	return fl.WriteFile(SCHEDDEV, b)
}

func (fl *FsLib) SpawnProgram(name string, args []string) error {
	a := &Attr{}
	a.Pid = GenPid()
	a.Program = name
	a.Args = args
	return fl.Spawn(a)
}

func (fl *FsLib) Started(pid string) error {
	return fl.WriteFile(SCHEDDEV, []byte("Started "+pid))
}

func (fl *FsLib) Exiting(pid string) error {
	return fl.WriteFile(SCHEDDEV, []byte("Exiting "+pid))
}

// The open blocks until pid exits (and then returns error, which is ignored)
func (fl *FsLib) Wait(pid string) {
	fl.Open(SCHED+"/Wait-"+pid, np.OREAD)
}
