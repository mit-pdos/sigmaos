package twopc

import (
	"ulambda/fslib"
	"ulambda/proc"
)

type TxnI interface {
	Prepare() error
	Commit() error
	Abort() error
	Done()
}

type Tstatus int

const (
	TINIT   Tstatus = 0
	TCOMMIT Tstatus = 1
	TABORT  Tstatus = 2
	TCRASH  Tstatus = 3
)

func (s Tstatus) String() string {
	switch s {
	case TINIT:
		return "INIT"
	case TCOMMIT:
		return "COMMIT"
	case TABORT:
		return "ABORT"
	default:
		return "CRASH"
	}
}

type Twopc struct {
	Tid          int
	Participants []string
	Status       Tstatus
}

func makeTwopc(tid int, ps []string) *Twopc {
	cf := &Twopc{tid, ps, TINIT}
	return cf
}

func readTwopc(fsl *fslib.FsLib, twopcfile string) *Twopc {
	twopc := Twopc{}
	err := fsl.ReadFileJson(twopcfile, &twopc)
	if err != nil {
		return nil
	}
	return &twopc
}

func SpawnCoord(pctl *proc.ProcCtl, opcode string, ps []string) string {
	args := append([]string{opcode}, ps...)
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/coord"
	a.Args = args
	a.StartDep = nil
	a.ExitDep = nil
	pctl.Spawn(&a)
	return a.Pid
}

func clean(fsl *fslib.FsLib) *Twopc {
	twopc := readTwopc(fsl, TWOPCPREP)
	if twopc == nil {
		twopc = readTwopc(fsl, TWOPCCOMMIT)
	}
	return twopc
}
