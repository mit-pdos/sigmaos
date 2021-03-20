package perf

import (
	"errors"
	"log"
	"strconv"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
)

type Spinner struct {
	pid  string
	msec int
	*fslib.FsLib
}

func MakeSpinner(args []string) (*Spinner, error) {
	if len(args) != 2 {
		return nil, errors.New("MakeSpinner: too few arguments")
	}

	s := &Spinner{}
	s.FsLib = fslib.MakeFsLib("spinner")
	s.pid = args[0]
	msec, err := strconv.Atoi(args[1])
	s.msec = msec
	if err != nil {
		log.Fatalf("Invalid sleep duration: %v, %v\n", args[1], err)
	}

	db.SetDebug()

	err = s.Started(s.pid)
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}

	return s, nil
}

func (s *Spinner) Work() {
	start := time.Now()
	for {
		if time.Since(start).Milliseconds() >= int64(s.msec) {
			break
		}
	}
	err := s.Exiting(s.pid, "OK")
	if err != nil {
		log.Fatalf("Exit: error %v\n", err)
	}
}
