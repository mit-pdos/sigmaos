package fslib

import (
	"encoding/json"
	"math/rand"
	"strconv"
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

func (fl *FsLib) Started(pid string) error {
	return fl.WriteFile(SCHEDDEV, []byte("Started "+pid))
}

func (fl *FsLib) Exiting(pid string) error {
	return fl.WriteFile(SCHEDDEV, []byte("Exiting "+pid))
}
