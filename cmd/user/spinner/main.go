package main

import (
	"errors"
	"os"
	"path"
	"runtime"

	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v out\n", os.Args[0])
	}
	l, err := NewSpinner(os.Args[1:])
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	l.Work()
}

type Spinner struct {
	*sigmaclnt.SigmaClnt
	outdir string
	done   bool
}

func NewSpinner(args []string) (*Spinner, error) {
	if len(args) < 1 {
		return nil, errors.New("NewSpinner: too few arguments")
	}
	s := &Spinner{}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	s.SigmaClnt = sc
	s.outdir = args[0]

	db.DPrintf(db.SPINNER, "NewSpinner: %v\n", args)

	ch := make(chan bool)
	// Start a goroutine to create the leased file
	go s.putFileWatch(ch)
	<-ch

	err = s.Started()
	if err != nil {
		db.DFatalf("Started: error %v\n", err)
	}
	return s, nil
}

func (s *Spinner) putFileWatch(ch chan bool) {
	li, err := s.LeaseClnt.AskLease(s.outdir, fsetcd.LeaseTTL)
	if err != nil {
		db.DFatalf("Error AskLease: %v", err)
	}
	li.KeepExtending()

	pn := path.Join(s.outdir, s.ProcEnv().GetPID().String())
	if _, err := s.PutLeasedFile(pn, 0777, sp.OWRITE, li.Lease(), []byte{}); err != nil {
		db.DFatalf("NewFile error: %v", err)
	}
	ch <- true
	close(ch)
	// Wait for the leased file to be removed:55

	if err := s.WaitRemove(pn); err != nil && !s.done {
		db.DFatalf("Err WaitRemove %v: %v", pn, err)
	}
}

func (s *Spinner) waitEvict() {
	err := s.WaitEvict(s.ProcEnv().GetPID())
	if err != nil {
		db.DFatalf("Error WaitEvict: %v", err)
	}
	s.done = true
	// Remove file
	s.Remove(path.Join(s.outdir, s.ProcEnv().GetPID().String()))
	s.ClntExit(proc.NewStatus(proc.StatusEvicted))
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
