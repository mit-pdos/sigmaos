package perf

import (
	"errors"
	"log"
	"strconv"

	// db "ulambda/debug"
	"ulambda/fslib"
)

type Spinner struct {
	pid string
	dim int
	its int
	*fslib.FsLib
}

func MakeSpinner(args []string) (*Spinner, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeSpinner: too few arguments")
	}

	s := &Spinner{}
	s.FsLib = fslib.MakeFsLib("spinner")
	s.pid = args[0]
	dim, err := strconv.Atoi(args[1])
	s.dim = dim
	if err != nil {
		log.Fatalf("Invalid dimension: %v, %v\n", args[1], err)
	}

	its, err := strconv.Atoi(args[2])
	s.its = its
	if err != nil {
		log.Fatalf("Invalid num interations: %v, %v\n", args[2], err)
	}

	err = s.Started(s.pid)
	if err != nil {
		log.Fatalf("Started: error %v\n", err)
	}

	return s, nil
}

func (s *Spinner) Work() {

	// Create and fill matrices
	A := MakeMatrix(s.dim, s.dim)
	B := MakeMatrix(s.dim, s.dim)
	C := MakeMatrix(s.dim, s.dim)
	A.FillRandomNonZero()
	B.FillRandomNonZero()
	C.Fill(0.0)

	//	compStart := time.Now()
	for i := 0; i < s.its; i++ {
		err := Mult(A, B, C)
		if err != nil {
			log.Fatalf("Error in matrix multiply: %v", err)
		}
	}
	//	compEnd := time.Now()
	//	e2eEnd := time.Now()

	// Calculate elapsed time
	//	compElapsed := compEnd.Sub(compStart)
	//	e2eElapsed := e2eEnd.Sub(e2eStart)
	//	log.Printf("Total elapsed computation time: %v msec(s)\n", compElapsed.Milliseconds())
	//	log.Printf("Average computation time: %v msec(s)\n", compElapsed.Milliseconds()/int64(s.its))
	//	log.Printf("Total elapsed setup time: %v msec(s)\n", e2eElapsed.Milliseconds()-compElapsed.Milliseconds())

	err := s.Exiting(s.pid, "OK")
	if err != nil {
		log.Fatalf("Exit: error %v\n", err)
	}
}
