package fslib

import (
	"math/rand"
	"strconv"
)

type PDep struct {
	Producer string
	Consumer string
}

type Ttype uint32
type Tcore uint32

const (
	T_DEF Ttype = 0
	T_LC  Ttype = 1
	T_BE  Ttype = 2
)

const (
	C_DEF Tcore = 0
)

type Attr struct {
	Pid     string
	Program string
	Dir     string
	Args    []string
	Env     []string
	PairDep []PDep
	ExitDep map[string]bool
	Timer   uint32
	Type    Ttype
	Ncore   Tcore
}

type WaitFile struct {
	Started      bool
	RetStatFiles []string
}

const (
	LOCALD_ROOT  = "name/localds"
	NO_OP_LAMBDA = "no-op-lambda"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}

func (fl *FsLib) HasBeenSpawned(pid string) bool {
	_, err := fl.Stat(waitFilePath(pid))
	if err == nil {
		return true
	}
	return false
}
