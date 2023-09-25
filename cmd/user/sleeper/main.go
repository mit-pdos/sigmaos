package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sessp"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	execTimeStr := os.Getenv("SIGMA_EXEC_TIME")
	execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
	if err != nil {
		db.DFatalf("Error parsing exec time: %v", err)
	}
	execTime := time.UnixMicro(execTimeMicro)
	db.DPrintf(db.SPAWN_LAT, "[%v] Proc exec latency: %v", proc.GetSigmaDebugPid(), time.Since(execTime))
	pe := proc.GetProcEnv()
	db.DPrintf(db.SPAWN_LAT, "[%v] E2e latency until main: %v", pe.GetPID(), time.Since(pe.GetSpawnTime()))
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v sleep_length outdir <native>\n", os.Args[0])
		os.Exit(1)
	}
	l, err := NewSleeper(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	l.Work()
	db.DPrintf(db.SLEEPER, "sleeper exit\n")
	os.Exit(0)
}

type Sleeper struct {
	*sigmaclnt.SigmaClnt
	native      bool
	sleepLength time.Duration
	outdir      string
	startSeqno  sessp.Tseqno
	time.Time
}

func NewSleeper(args []string) (*Sleeper, error) {
	if len(args) < 2 {
		return nil, errors.New("NewSleeper: too few arguments")
	}
	s := &Sleeper{}
	s.Time = time.Now()
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClient: %v", err)
	}
	s.SigmaClnt = sc
	s.startSeqno = s.ReadSeqNo()
	s.outdir = args[1]
	d, err := time.ParseDuration(args[0])
	if err != nil {
		db.DFatalf("Error parsing duration: %v", err)
	}
	s.sleepLength = d

	s.native = len(args) == 3 && args[2] == "native"

	db.DPrintf(db.SLEEPER, "NewSleeper: %v\n", args)

	if !s.native {
		err := s.Started()
		if err != nil {
			db.DFatalf("Started: error %v", err)
		}
	}
	return s, nil
}

func (s *Sleeper) waitEvict(ch chan *proc.Status) {
	if !s.native {
		err := s.WaitEvict(s.ProcEnv().GetPID())
		if err != nil {
			db.DFatalf("Error WaitEvict: %v", err)
		}
		ch <- proc.NewStatus(proc.StatusEvicted)
	}
}

func (s *Sleeper) sleep(ch chan *proc.Status) {
	time.Sleep(s.sleepLength)
	if s.outdir != "" {
		fpath := path.Join(s.outdir, s.ProcEnv().GetPID().String()+"_out")
		_, err := s.PutFile(fpath, 0777, sp.OWRITE, []byte("hello"))
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error: Newfile %v in Sleeper.Work: %v\n", fpath, err)
		}
	}
	// Measure latency of all ops except for Exited.
	ch <- proc.NewStatusInfo(proc.StatusOK, "elapsed time", time.Since(s.Time))
}

func (s *Sleeper) Work() {
	ch := make(chan *proc.Status)
	go s.waitEvict(ch)
	go s.sleep(ch)
	status := <-ch
	if !s.native {
		start := time.Now()
		s.ClntExit(status)
		db.DPrintf(db.SLEEPER_TIMING, "Elapsed %v us", time.Since(start).Microseconds())
	}
}
