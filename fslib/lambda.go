package fslib

import (
	"encoding/json"
	"math/rand"
	"path"
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
	Dir     string
	Args    []string
	Env     []string
	PairDep []PDep
	ExitDep []string
}

const (
	SCHED        = "name/schedd"
	LOCALD_ROOT  = "name/localds"
	SCHEDDEV     = SCHED + "/" + "dev"
	NO_OP_LAMBDA = "no-op-lambda"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}

// XXX Do we have to worry about lack of atomicity between read and write?
func (fl *FsLib) SwapExitDependency(pids []string) error {
	b := strings.Join(pids, " ")
	ls, _ := fl.ReadDir(SCHED)
	for _, l := range ls {
		err := fl.WriteFile(SCHED+"/"+l.Name+"/ExitDep", []byte(b))
		if err != nil {
			// XXX ignore for now... lambda may have exited, in which case we get an
			// error
		}
	}
	return nil
}

// Spawn a new lambda
func (fl *FsLib) Spawn(a *Attr) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return fl.MakeFile(SCHED+"/"+a.Pid, b)
}

// XXX Rename
func (fl *FsLib) RunLocal(ip string, a *Attr) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return fl.MakeFile(path.Join(LOCALD_ROOT, ip, a.Pid), b)
}

func (fl *FsLib) SpawnProgram(name string, args []string) error {
	a := &Attr{}
	a.Pid = GenPid()
	a.Program = name
	a.Args = args
	return fl.Spawn(a)
}

// Spawn a no-op lambda
func (fl *FsLib) SpawnNoOp(pid string, exitDep []string) error {
	a := &Attr{}
	a.Pid = pid
	a.Program = NO_OP_LAMBDA
	a.ExitDep = exitDep
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
