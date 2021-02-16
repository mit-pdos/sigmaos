package fslib

import (
	"encoding/json"
	"math/rand"
	"strconv"
	"strings"
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

func (fl *FsLib) SwapExitDependencies(pids []string) error {
	b := strings.Join(pids, " ")
	return fl.WriteFile(SCHEDDEV, []byte("SwapExitDependencies "+b))
}

// Spawn a new  lambda
func (fl *FsLib) Spawn(a *Attr) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return fl.MakeFile(SCHED+"/"+a.Pid, b)
}

// Continuate a.pid later
func (fl *FsLib) Continue(a *Attr) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return fl.WriteFile(SCHED+"/"+a.Pid, b)
}

func (fl *FsLib) SpawnProgram(name string, args []string) error {
	a := &Attr{}
	a.Pid = GenPid()
	a.Program = name
	a.Args = args
	return fl.Spawn(a)
}

func (fl *FsLib) Started(pid string) error {
	return fl.WriteFile(SCHED+"/"+pid+"/Status", []byte{})
}

func (fl *FsLib) Exiting(pid string, status string) error {
	return fl.WriteFile(SCHED+"/"+pid+"/ExitStatus", []byte(status))
}

// The open blocks until pid exits and then reads ExitStatus
func (fl *FsLib) Wait(pid string) ([]byte, error) {
	return fl.ReadFile(SCHED + "/" + pid + "/ExitStatus")
}
