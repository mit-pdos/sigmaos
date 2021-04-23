package twopc

import (
	"log"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
)

type TxnTest struct {
	*fslib.FsLib
	pid    string
	me     string
	flwr   string
	index  string
	opcode string
	args   []string
}

type Tinput struct {
	Fns []string
}

func txnname(pid string) string {
	return "txn" + pid
}

func MkTxnTest(args []string) (*TxnTest, error) {
	txn := &TxnTest{}
	txn.pid = args[0]
	txn.me = txnname(txn.pid)
	txn.flwr = args[1]
	txn.index = args[2]
	txn.opcode = args[3]
	db.Name(txn.me)
	txn.FsLib = fslib.MakeFsLib(txn.me)
	return txn, nil
}

func (txn *TxnTest) copyFile(fn1, fn2 string) error {
	b, err := txn.ReadFile(fn1)
	if err != nil {
		log.Fatalf("ReadFile %v err %v\n", fn1, err)
	}
	err = txn.MakeFile(fn2, 0777, b)
	if err != nil {
		log.Fatalf("MakeFile %v err %v\n", fn2, err)
	}
	return nil
}

func (txn *TxnTest) prepare(ti *Tinput) {
	var err error
	switch txn.index {
	case "0":
		err = txn.copyFile(ti.Fns[0]+"x", ti.Fns[1]+"x#")
	case "1":
		err = txn.copyFile(ti.Fns[1]+"y", ti.Fns[2]+"y#")
	default:
	}
	if err != nil {
		log.Fatalf("prepare: failed %v\n", err)
	}
}

func (txn *TxnTest) commit(ti *Tinput) {
	var err error
	switch txn.index {
	case "0":
		txn.Rename(ti.Fns[1]+"x#", ti.Fns[1]+"x")
	case "1":
		txn.Rename(ti.Fns[2]+"y#", ti.Fns[2]+"y")
	default:
	}
	if err != nil {
		log.Fatalf("commit: failed %v\n", err)
	}
}

func (txn *TxnTest) abort(ti *Tinput) {
	var err error
	switch txn.index {
	case "0":
		txn.Remove(ti.Fns[1] + "x#")
	case "1":
		txn.Remove(ti.Fns[2] + "y#")
	default:
	}
	if err != nil {
		log.Fatalf("abort: failed %v\n", err)
	}
}

func (txn *TxnTest) Run() {
	log.Printf("%v: TxnTest %v i %v op %v\n", txn.me, txn.flwr, txn.index, txn.opcode)
	ti := Tinput{}
	err := txn.ReadFileJson(memfsd.MEMFS+"/txni", &ti)
	if err != nil {
		log.Fatalf("Failed to read txn %v\n", err)
	}
	log.Printf("ti %v\n", ti)
	switch txn.opcode {
	case "prepare":
		txn.prepare(&ti)
	case "commit":
		txn.commit(&ti)
	case "abort":
		txn.abort(&ti)
	default:
		log.Fatalf("Unknown upcode %v\n", txn.opcode)
	}
}
