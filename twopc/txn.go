package twopc

import (
	"log"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
)

type TxnNull struct {
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

func MkTxnTest(args []string) (*TxnNull, error) {
	txn := &TxnNull{}
	txn.pid = args[0]
	txn.me = txnname(txn.pid)
	txn.flwr = args[1]
	txn.index = args[2]
	txn.opcode = args[3]
	db.Name(txn.me)
	txn.FsLib = fslib.MakeFsLib(txn.me)
	return txn, nil
}

func (txn *TxnNull) Run() {
	log.Printf("%v: TxnTest %v i %v op %v\n", txn.me, txn.flwr, txn.index, txn.opcode)

	ti := Tinput{}
	err := txn.ReadFileJson(memfsd.MEMFS+"/txni", &ti)
	if err != nil {
		log.Fatalf("Failed to read txn %v\n", err)
	}
	log.Printf("ti %v\n", ti)
}
