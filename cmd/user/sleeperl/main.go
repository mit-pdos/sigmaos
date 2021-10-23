package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid sleep_length out <native>\n", os.Args[0])
		os.Exit(1)
	}
	l, err := MakeSleeperl(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	l.Work()
}

type Sleeperl struct {
	*fslib.FsLib
	proc.ProcClnt
	native      bool
	pid         string
	sleepLength time.Duration
	output      string
}

func MakeSleeperl(args []string) (*Sleeperl, error) {
	if len(args) < 3 {
		return nil, errors.New("MakeSleeperl: too few arguments")
	}
	s := &Sleeperl{}
	db.Name("sleeperl")
	s.FsLib = fslib.MakeFsLib("sleeperl")
	s.ProcClnt = procinit.MakeProcClnt(s.FsLib, procinit.GetProcLayersMap())
	s.pid = args[0]
	s.output = args[2]
	d, err := time.ParseDuration(args[1])
	if err != nil {
		log.Fatalf("Error parsing duration: %v", err)
	}
	s.sleepLength = d

	s.native = len(args) == 4 && args[3] == "native"

	db.DLPrintf("SCHEDL", "MakeSleeperl: %v\n", args)
	//	log.Printf("MakeSleeperl: %v\n", args)

	if !s.native {
		err := s.Started(s.pid)
		if err != nil {
			log.Fatalf("Started: error %v\n", err)
		}
	}
	return s, nil
}

func (s *Sleeperl) waitEvict() {
	if !s.native {
		err := s.WaitEvict(s.pid)
		if err != nil {
			log.Fatalf("Error WaitEvict: %v", err)
		}
		s.Exited(s.pid, "EVICTED")
		os.Exit(0)
	}
}

func (s *Sleeperl) Work() {
	go s.waitEvict()
	time.Sleep(s.sleepLength)
	err := s.MakeFile(s.output, 0777, np.OWRITE, []byte("hello"))
	if err != nil {
		log.Printf("Error: Makefile %v in Sleeperl.Work: %v\n", s.output, err)
	}
	if !s.native {
		s.Exited(s.pid, "OK")
	}
}
