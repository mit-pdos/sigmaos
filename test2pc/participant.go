package test2pc

import (
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/twopc"
)

type Tinput struct {
	Fns []string
}

type Part2pc struct {
	*fslib.FsLib
	pid    string
	me     string
	index  string
	opcode string
	args   []string
	done   chan bool
	ti     *Tinput
}

func partname(pid string) string {
	return "P" + pid
}

func MkTest2Participant(args []string) (*Part2pc, error) {
	p := &Part2pc{}
	p.done = make(chan bool)
	p.pid = args[0]
	p.me = partname(p.pid)
	p.index = args[1]
	p.opcode = args[2]
	db.Name(p.me)
	p.FsLib = fslib.MakeFsLib(p.me)

	log.Printf("%v: Part2pc i %v op %v\n", p.me, p.index, p.opcode)
	p.ti = &Tinput{}
	err := p.ReadFileJson(memfsd.MEMFS+"/txni", p.ti)
	if err != nil {
		log.Fatalf("Failed to read txni %v\n", err)
	}

	_, err = twopc.MakeParticipant(p.FsLib, p.me, p, p.opcode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}

	p.Started(p.pid)

	return p, nil
}

func (p *Part2pc) copyFile(fn1, fn2 string) error {
	b, err := p.ReadFile(fn1)
	if err != nil {
		log.Fatalf("ReadFile %v err %v\n", fn1, err)
	}
	err = p.MakeFile(fn2, 0777, np.OWRITE, b)
	if err != nil {
		log.Fatalf("MakeFile %v err %v\n", fn2, err)
	}
	return nil
}

func (p *Part2pc) Prepare() error {
	var err error
	switch p.index {
	case "0":
		err = p.copyFile(p.ti.Fns[0]+"x", p.ti.Fns[1]+"x#")
	case "1":
		err = p.copyFile(p.ti.Fns[1]+"y", p.ti.Fns[2]+"y#")
	default:
	}
	return err
}

func (p *Part2pc) Commit() error {
	var err error
	switch p.index {
	case "0":
		err = p.Rename(p.ti.Fns[1]+"x#", p.ti.Fns[1]+"x")
	case "1":
		err = p.Rename(p.ti.Fns[2]+"y#", p.ti.Fns[2]+"y")
	default:
	}
	return err
}

func (p *Part2pc) Abort() error {
	var err error
	switch p.index {
	case "0":
		err = p.Remove(p.ti.Fns[1] + "x#")
	case "1":
		err = p.Remove(p.ti.Fns[2] + "y#")
	default:
	}
	return err
}

func (p *Part2pc) Done() {
	p.done <- true
}

func (p *Part2pc) Work() {
	db.DLPrintf("TEST2PC", "Work\n")
	<-p.done
	db.DLPrintf("TEST2PC", "exit\n")

}
