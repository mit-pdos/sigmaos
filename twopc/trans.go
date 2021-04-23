package twopc

import (
	"ulambda/fslib"
)

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

type Trans struct {
	Tid       int
	Followers []string
	Status    Tstatus
}

func makeTrans(tid int, fws []string) *Trans {
	cf := &Trans{tid, fws, TINIT}
	return cf
}

func readTrans(fsl *fslib.FsLib, txnfile string) *Trans {
	txn := Trans{}
	err := fsl.ReadFileJson(txnfile, &txn)
	if err != nil {
		return nil
	}
	return &txn
}

func spawnCoord(fsl *fslib.FsLib, flwrs []string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/coord"
	a.Args = flwrs
	a.PairDep = nil
	a.ExitDep = nil
	fsl.Spawn(&a)
	return a.Pid
}
