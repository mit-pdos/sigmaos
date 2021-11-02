package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procinit"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid out\n", os.Args[0])
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
	proc.ProcClnt
	pid    string
	output string
}

func MakeSpinner(args []string) (*Spinner, error) {
	if len(args) < 2 {
		return nil, errors.New("MakeSpinner: too few arguments")
	}
	s := &Spinner{}
	db.Name("spinner")
	s.FsLib = fslib.MakeFsLib("spinner")
	s.ProcClnt = procinit.MakeProcClnt(s.FsLib, procinit.GetProcLayersMap())
	s.pid = args[0]
	s.output = args[1]

	db.DLPrintf("SCHEDL", "MakeSpinner: %v\n", args)

	err := s.Started(s.pid)
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}
	return s, nil
}

func (s *Spinner) waitEvict() {
	err := s.WaitEvict(s.pid)
	if err != nil {
		log.Fatalf("Error WaitEvict: %v", err)
	}
	s.Exited(s.pid, "EVICTED")
	os.Exit(0)
}

func (s *Spinner) Work() {
	go s.waitEvict()
	for {
	}
}

func (s *Spinner) Exit() {
	s.Exited(s.pid, "OK")
}
