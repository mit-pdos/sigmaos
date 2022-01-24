package main

import (
	"errors"
	"log"
	"os"
	"path"
	"runtime"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %v out\n", os.Args[0])
	}
	l, err := MakeSpinner(os.Args[1:])
	if err != nil {
		log.Fatalf("%v: error %v", os.Args[0], err)
	}
	l.Work()
}

type Spinner struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	outdir string
}

func MakeSpinner(args []string) (*Spinner, error) {
	if len(args) < 1 {
		return nil, errors.New("MakeSpinner: too few arguments")
	}
	s := &Spinner{}
	db.Name("spinner")
	s.FsLib = fslib.MakeFsLib("spinner")
	s.ProcClnt = procclnt.MakeProcClnt(s.FsLib)
	s.outdir = args[0]

	db.DLPrintf("SCHEDL", "MakeSpinner: %v\n", args)

	if err := s.MakeFile(path.Join(s.outdir, proc.GetPid()), 0777|np.DMTMP, np.OWRITE, []byte{}); err != nil {
		log.Fatalf("MakeFile error: %v", err)
	}

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

func (s *Spinner) spin() {
	for {
		runtime.Gosched()
	}
}

func (s *Spinner) Work() {
	go s.spin()
	s.waitEvict()
}
