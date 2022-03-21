package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
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
	*fslib.FsLib
	*procclnt.ProcClnt
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
	s.FsLib = fslib.MakeFsLib("spawner-" + proc.GetPid().String())
	s.ProcClnt = procclnt.MakeProcClnt(s.FsLib)
	b, err := strconv.ParseBool(args[0])
	if err != nil {
		log.Fatalf("Error parseBool: %v %v", args[0], err)
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
		log.Fatalf("Error spawn: %v", err)
	}
	s.Started()
	if s.shouldWaitExit {
		s.WaitExit(s.childPid)
	}
	s.Exited(proc.MakeStatus(proc.StatusOK))
}
