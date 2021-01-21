package fslib

import (
	"encoding/json"
)

type PDep struct {
	Producer string
	Consumer string
}

type Attr struct {
	Pid     string
	Program string
	Args    []string
	PairDep []PDep
	ExitDep []string
}

const (
	SCHED    = "name/schedd"
	SDEV     = "schedd"
	SCHEDDEV = SCHED + "/" + SDEV
)

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
