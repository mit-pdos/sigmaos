package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v out\n", os.Args[0])
		os.Exit(1)
	}
	l, err := MakeSpinner(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	l.Work()
	l.Exit()
}

type Spinner struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	output string
}

func MakeSpinner(args []string) (*Spinner, error) {
	if len(args) < 1 {
		return nil, errors.New("MakeSpinner: too few arguments")
	}
	s := &Spinner{}
	db.Name("spinner")
	s.FsLib = fslib.MakeFsLib("spinner")
	s.ProcClnt = procclnt.MakeProcClnt(s.FsLib)
	s.output = args[0]

	db.DLPrintf("SCHEDL", "MakeSpinner: %v\n", args)

	err := s.Started(proc.GetPid())
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}
	return s, nil
}

func (s *Spinner) waitEvict() {
	err := s.WaitEvict(proc.GetPid())
	if err != nil {
		log.Fatalf("Error WaitEvict: %v", err)
	}
	s.Exited(proc.GetPid(), "EVICTED")
	os.Exit(0)
}

func (s *Spinner) Work() {
	go s.waitEvict()
	for {
	}
}

func (s *Spinner) Exit() {
	s.Exited(proc.GetPid(), "OK")
}
