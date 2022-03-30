package main

import (
	"errors"
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
		db.DFatalf("Usage: %v out\n", os.Args[0])
	}
	l, err := MakeSpinner(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
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
	s.FsLib = fslib.MakeFsLib("spinner")
	s.ProcClnt = procclnt.MakeProcClnt(s.FsLib)
	s.outdir = args[0]

	db.DPrintf("SCHEDL", "MakeSpinner: %v\n", args)

	if _, err := s.PutFile(path.Join(s.outdir, proc.GetPid().String()), 0777|np.DMTMP, np.OWRITE, []byte{}); err != nil {
		db.DFatalf("MakeFile error: %v", err)
	}

	err := s.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	return s, nil
}

func (s *Spinner) waitEvict() {
	err := s.WaitEvict(proc.GetPid())
	if err != nil {
		db.DFatalf("Error WaitEvict: %v", err)
	}
	s.Exited(proc.MakeStatus(proc.StatusEvicted))
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
