package schedd

import (
	"errors"
	"log"
	"time"

	// db "ulambda/debug"
	"ulambda/fslib"
)

type Schedl struct {
	*fslib.FsLib
	pid    string
	output string
}

func MakeSchedl(args []string) (*Schedl, error) {
	if len(args) != 2 {
		return nil, errors.New("MakeSchedl: too few arguments")
	}
	log.Printf("MakeSchedl: %v\n", args)

	s := &Schedl{}
	s.FsLib = fslib.MakeFsLib("schedl")
	s.pid = args[0]
	s.output = args[1]

	err := s.Started(s.pid)
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}

	return s, nil
}

func (s *Schedl) Work() {

	time.Sleep(10000 * time.Millisecond)

	err := s.MakeFile(s.output, []byte("hello"))
	if err != nil {
		log.Printf("Makefile error %v\n", err)
	}

	err = s.Exiting(s.pid, "OK")
	if err != nil {
		log.Fatalf("Exit: error %v\n", err)
	}
}
