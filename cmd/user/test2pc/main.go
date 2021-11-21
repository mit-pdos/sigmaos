package main

import (
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/twopc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v index opcode\n", os.Args[0])
		os.Exit(1)
	}
	p, err := MkTest2Participant(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	p.Work()
}

type Tinput struct {
	Fns []string
}

type Part2pc struct {
	*fslib.FsLib
	*procclnt.ProcClnt
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
	p.me = partname(proc.GetPid())
	p.index = args[0]
	p.opcode = args[1]
	db.Name(p.me)
	p.FsLib = fslib.MakeFsLib(p.me)
	p.ProcClnt = procclnt.MakeProcClnt(p.FsLib)

	log.Printf("%v: Part2pc i %v op %v\n", p.me, p.index, p.opcode)
	p.ti = &Tinput{}
	err := p.ReadFileJson(named.MEMFS+"/txni", p.ti)
	if err != nil {
		log.Fatalf("Failed to read txni %v\n", err)
	}

	_, err = twopc.MakeParticipant(p.FsLib, p.me, p, p.opcode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}

	p.Started(proc.GetPid())

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
	p.Exited(proc.GetPid(), "OK")
}
