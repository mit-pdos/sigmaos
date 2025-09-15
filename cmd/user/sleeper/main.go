package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"os/exec"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

func main() {
	if os.Getenv("IS_FORKTEST") == "true" {
		if len(os.Args) == 1 {
			db.DPrintf(db.ALWAYS, "In parent")
			cmd := exec.Command("./bin", []string{strconv.FormatInt(time.Now().UnixMicro(), 10)}...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				db.DFatalf("Error start %v %v", cmd, err)
			}
			if err := cmd.Wait(); err != nil {
				db.DFatalf("Error wait %v %v", cmd, err)
			}
			return
		} else {
			execTimeStr := os.Args[1]
			execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
			if err != nil {
				db.DFatalf("Error parsing exec time 1: %v", err)
			}
			execTime := time.UnixMicro(execTimeMicro)
			db.DPrintf(db.ALWAYS, "Trampoline exec latency: %v", time.Since(execTime))
			db.DPrintf(db.ALWAYS, "In child")
			return
		}
	}
	execTimeStr := os.Getenv("SIGMA_EXEC_TIME")
	execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
	if err != nil {
		db.DFatalf("Error parsing exec time 2: %v", err)
	}
	execTime := time.UnixMicro(execTimeMicro)
	pe := proc.GetProcEnv()
	perf.LogSpawnLatency("SpawnBench.Exec", pe.GetPID(), pe.GetSpawnTime(), execTime)
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
	db.DPrintf(db.SLEEPER, "Pre sleep")
	time.Sleep(s.sleepLength)
	db.DPrintf(db.SLEEPER, "Post sleep")
	if s.outdir != "" {
		fpath := path.Join(s.outdir, s.ProcEnv().GetPID().String()+"_out")
		db.DPrintf(db.SLEEPER, "PutFile")
		_, err := s.PutFile(fpath, 0777, sp.OWRITE, []byte("hello"))
		if err != nil {
			db.DPrintf(db.SLEEPER, "Error: Newfile %v in Sleeper.Work: %v\n", fpath, err)
		}
		db.DPrintf(db.SLEEPER, "PutFile done")
	}
	db.DPrintf(db.SLEEPER, "Send on channel")
	// Measure latency of all ops except for Exited.
	ch <- proc.NewStatusInfo(proc.StatusOK, "elapsed time", time.Since(s.Time))
}

func (s *Sleeper) Work() {
	ch := make(chan *proc.Status)
	go s.waitEvict(ch)
	go s.sleep(ch)
	status := <-ch
	db.DPrintf(db.SLEEPER, "Recv status %v", status)
	if !s.native {
		start := time.Now()
		db.DPrintf(db.SLEEPER, "Exiting")
		s.ClntExit(status)
		db.DPrintf(db.SLEEPER, "Exiting done")
		db.DPrintf(db.SLEEPER_TIMING, "Elapsed %v us", time.Since(start).Microseconds())
	}
}
