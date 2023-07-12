package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v shouldWaitExit child_pid child_program child_args... \n", os.Args[0])
		os.Exit(1)
	}
	l, err := MakeSpawner(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	l.Work()
}

type Spawner struct {
	*sigmaclnt.SigmaClnt
	shouldWaitExit bool
	childPid       proc.Tpid
	childProgram   string
	childArgs      []string
}

func MakeSpawner(args []string) (*Spawner, error) {
	if len(args) < 3 {
		return nil, errors.New("MakeSpawner: too few arguments")
	}
	// 	log.Printf("MakeSpawner %v", args)
	s := &Spawner{}
	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname("spawner-" + proc.GetPid().String()))
	if err != nil {
		return nil, err
	}
	s.SigmaClnt = sc
	b, err := strconv.ParseBool(args[0])
	if err != nil {
		db.DFatalf("Error parseBool: %v %v", args[0], err)
	}
	s.shouldWaitExit = b
	s.childPid = proc.Tpid(args[1])
	s.childProgram = args[2]
	s.childArgs = args[3:]

	return s, nil
}

func (s *Spawner) Work() {
	p := proc.MakeProcPid(s.childPid, s.childProgram, s.childArgs)
	err := s.Spawn(p)
	if err != nil {
		db.DFatalf("Error spawn: %v", err)
	}
	s.Started()
	crash.Crasher(s.FsLib)
	if s.shouldWaitExit {
		s.WaitExit(s.childPid)
	}
	s.ExitOK()
}
