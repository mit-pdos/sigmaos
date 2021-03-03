package schedd

import (
	"errors"
	"log"
	"strconv"

	// db "ulambda/debug"
	"ulambda/fslib"
)

type Schedl struct {
	*fslib.FsLib
	pid    string
	n      int
	output string
}

func MakeSchedl(args []string) (*Schedl, error) {
	if len(args) != 4 {
		return nil, errors.New("MakeSchedl: too few arguments")
	}
	log.Printf("MakeSchedl: %v\n", args)

	s := &Schedl{}
	s.FsLib = fslib.MakeFsLib("schedl")
	s.pid = args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("Strconv error %v\n", err)
	}
	s.n = n
	s.output = args[2]

	err = s.Started(s.pid)
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}

	return s, nil
}

func (s *Schedl) Work() {
	//	b, err := s.ReadFile(s.output)
	//	if err != nil {
	//		log.Fatalf("schedl: read failed %v\n", err)
	//	}
	//	if string(b) != "hello" {
	//		log.Fatalf("schedl: read returned %v\n", string(b))
	//	}
	//	err = s.Exiting(s.pid, "OK")
	//	if err != nil {
	//		log.Fatalf("Exit: error %v\n", err)
	//	}
	//	err = s.WriteFile(s.output, []byte("hello again"))
	//	if err != nil {
	//		log.Fatalf("schedl: write failed %v\n", err)
	//	}
	//	time.Sleep(5000 * time.Millisecond)
	err := s.MakeFile(s.output, []byte("hello"))
	if err != nil {
		log.Printf("Makefile error %v\n", err)
	}

	err = s.Exiting(s.pid, "OK")
	if err != nil {
		log.Fatalf("Exit: error %v\n", err)
	}
	s.n -= 1
	n := strconv.Itoa(s.n)
	pid := fslib.GenPid()
	a := &fslib.Attr{pid, "../bin/schedl", "", []string{n, s.output, ""},
		nil, nil, nil}
	err = s.Spawn(a)
	if err != nil {
		log.Fatalf("Spawn: error %v\n", err)
	}

	log.Printf("schedl %v: spawned %v\n", s.pid, a)
}
