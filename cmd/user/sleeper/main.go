package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v sleep_length outdir <native>\n", os.Args[0])
		os.Exit(1)
	}
	l, err := MakeSleeper(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	l.Work()
}

type Sleeper struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	native      bool
	sleepLength time.Duration
	outdir      string
	startSeqno  np.Tseqno
	time.Time
}

func MakeSleeper(args []string) (*Sleeper, error) {
	if len(args) < 2 {
		return nil, errors.New("MakeSleeper: too few arguments")
	}
	s := &Sleeper{}
	s.Time = time.Now()
	s.FsLib = fslib.MakeFsLib("sleeper-" + proc.GetPid().String())
	s.ProcClnt = procclnt.MakeProcClnt(s.FsLib)
	s.startSeqno = s.ReadSeqNo()
	s.outdir = args[1]
	d, err := time.ParseDuration(args[0])
	if err != nil {
		db.DFatalf("Error parsing duration: %v", err)
	}
	s.sleepLength = d

	s.native = len(args) == 3 && args[2] == "native"

	db.DPrintf("PROCD", "MakeSleeper: %v\n", args)

	if !s.native {
		err := s.Started()
		if err != nil {
			db.DFatalf("%v: Started: error %v\n", proc.GetName(), err)
		}
	}
	return s, nil
}

func (s *Sleeper) waitEvict(ch chan *proc.Status) {
	if !s.native {
		err := s.WaitEvict(proc.GetPid())
		if err != nil {
			db.DFatalf("Error WaitEvict: %v", err)
		}
		ch <- proc.MakeStatus(proc.StatusEvicted)
	}
}

func (s *Sleeper) sleep(ch chan *proc.Status) {
	time.Sleep(s.sleepLength)
	if s.outdir != "" {
		fpath := path.Join(s.outdir, proc.GetPid().String()+"_out")
		_, err := s.PutFile(fpath, 0777, np.OWRITE, []byte("hello"))
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error: Makefile %v in Sleeper.Work: %v\n", fpath, err)
		}
	}
	latency := time.Since(s.Time)
	// Measure latency & NRPC of all ops except for Exited.
	res := benchmarks.MakeResult()
	res.Throughput = 0.0
	res.Latency = float64(latency.Microseconds())
	res.NRPC = s.ReadSeqNo()
	ch <- proc.MakeStatusInfo(proc.StatusOK, "elapsed time", res)
}

func (s *Sleeper) Work() {
	ch := make(chan *proc.Status)
	go s.waitEvict(ch)
	go s.sleep(ch)
	status := <-ch
	if !s.native {
		start := time.Now()
		s.Exited(status)
		db.DPrintf("TIMING", "Elapsed %v us", time.Since(start).Microseconds())
	}
}
