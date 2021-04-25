package test2pc

import (
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	"ulambda/twopc"
)

type Tinput struct {
	Fns []string
}

type Flwr2pc struct {
	*fslib.FsLib
	pid    string
	me     string
	flwr   string
	index  string
	opcode string
	args   []string
	done   chan bool
	ti     *Tinput
}

func flwname(pid string) string {
	return "flw" + pid
}

func MkTest2pc(args []string) (*Flwr2pc, error) {
	flw := &Flwr2pc{}
	flw.done = make(chan bool)
	flw.pid = args[0]
	flw.me = flwname(flw.pid)
	flw.index = args[1]
	flw.opcode = args[2]
	db.Name(flw.me)
	flw.FsLib = fslib.MakeFsLib(flw.me)

	log.Printf("%v: Flwr2pc i %v op %v\n", flw.me, flw.index, flw.opcode)
	flw.ti = &Tinput{}
	err := flw.ReadFileJson(memfsd.MEMFS+"/txni", flw.ti)
	if err != nil {
		log.Fatalf("Failed to read txni %v\n", err)
	}
	log.Printf("ti %v\n", flw.ti)

	_, err = twopc.MakeFollower(flw.FsLib, flw.me, flw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}

	flw.Started(flw.pid)

	return flw, nil
}

func (flw *Flwr2pc) copyFile(fn1, fn2 string) error {
	b, err := flw.ReadFile(fn1)
	if err != nil {
		log.Fatalf("ReadFile %v err %v\n", fn1, err)
	}
	err = flw.MakeFile(fn2, 0777, b)
	if err != nil {
		log.Fatalf("MakeFile %v err %v\n", fn2, err)
	}
	return nil
}

func (flw *Flwr2pc) Prepare() error {
	var err error
	switch flw.index {
	case "0":
		err = flw.copyFile(flw.ti.Fns[0]+"x", flw.ti.Fns[1]+"x#")
	case "1":
		err = flw.copyFile(flw.ti.Fns[1]+"y", flw.ti.Fns[2]+"y#")
	default:
	}
	return err
}

func (flw *Flwr2pc) Commit() error {
	var err error
	switch flw.index {
	case "0":
		err = flw.Rename(flw.ti.Fns[1]+"x#", flw.ti.Fns[1]+"x")
	case "1":
		err = flw.Rename(flw.ti.Fns[2]+"y#", flw.ti.Fns[2]+"y")
	default:
	}
	return err
}

func (flw *Flwr2pc) Abort() error {
	var err error
	switch flw.index {
	case "0":
		err = flw.Remove(flw.ti.Fns[1] + "x#")
	case "1":
		err = flw.Remove(flw.ti.Fns[2] + "y#")
	default:
	}
	return err
}

func (flw *Flwr2pc) Done() {
	flw.done <- true
}

func (flw *Flwr2pc) Work() {
	db.DLPrintf("TEST2PC", "Work\n")
	<-flw.done
	db.DLPrintf("TEST2PC", "exit\n")

}
