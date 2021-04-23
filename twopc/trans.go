package twopc

import (
	"strconv"

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
	Prog      string
}

func (txn *Trans) Index(me string) string {
	for i, fw := range txn.Followers {
		if fw == me {
			return strconv.Itoa(i)
		}
	}
	return ""
}

func makeTrans(tid int, fws []string, prog string) *Trans {
	cf := &Trans{tid, fws, TINIT, prog}
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

func spawnCoord(fsl *fslib.FsLib, opcode string, prog string, flwrs []string) string {
	args := append([]string{opcode, prog}, flwrs...)
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = "bin/coord"
	a.Args = args
	a.PairDep = nil
	a.ExitDep = nil
	fsl.Spawn(&a)
	return a.Pid
}

func spawnTrans(fsl *fslib.FsLib, prog, flwr, index, opcode string) string {
	a := fslib.Attr{}
	a.Pid = fslib.GenPid()
	a.Program = prog
	a.Args = []string{flwr, index, opcode}
	a.PairDep = nil
	a.ExitDep = nil
	fsl.Spawn(&a)
	return a.Pid
}

func clean(fsl *fslib.FsLib) *Trans {
	txn := readTrans(fsl, TXNPREP)
	if txn == nil {
		txn = readTrans(fsl, TXNCOMMIT)
	}
	return txn
}
