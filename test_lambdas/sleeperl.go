package test_lambdas

import (
	"errors"
	"log"
	"os"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
)

type Sleeperl struct {
	*fslib.FsLib
	proc.ProcClnt
	pid         string
	sleepLength time.Duration
	output      string
}

func MakeSleeperl(args []string) (*Sleeperl, error) {
	if len(args) != 4 {
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

	db.DLPrintf("SCHEDL", "MakeSleeperl: %v\n", args)
	//	log.Printf("MakeSleeperl: %v\n", args)

	err = s.Started(s.pid)
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}

	return s, nil
}

func (s *Sleeperl) waitEvict() {
	err := s.WaitEvict(s.pid)
	if err != nil {
		log.Fatalf("Error WaitEvict: %v", err)
	}
	s.Exit()
	os.Exit(0)
}

func (s *Sleeperl) Work() {
	go s.waitEvict()
	time.Sleep(s.sleepLength)
	err := s.MakeFile(s.output, 0777, np.OWRITE, []byte("hello"))
	if err != nil {
		log.Printf("Error: Makefile in Sleeperl.Work: %v\n", err)
	}
}

func (s *Sleeperl) Exit() {
	s.Exited(s.pid, "OK")
}
