package test_lambdas

import (
	"errors"
	"log"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type Sleeperl struct {
	*fslib.FsLib
	pid    string
	output string
}

func MakeSleeperl(args []string) (*Sleeperl, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeSleeperl: too few arguments")
	}
	s := &Sleeperl{}
	db.Name("sleeperl")
	s.FsLib = fslib.MakeFsLib("sleeperl")
	s.pid = args[0]
	s.output = args[1]

	db.DLPrintf("SCHEDL", "MakeSleeperl: %v\n", args)
	log.Printf("MakeSleeperl: %v\n", args)

	err := s.Started(s.pid)
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}

	return s, nil
}

func (s *Sleeperl) Work() {
	time.Sleep(5000 * time.Millisecond)
	err := s.MakeFile(s.output, 0777, np.OWRITE, []byte("hello"))
	if err != nil {
		log.Printf("Makefile error %v\n", err)
	}
}
