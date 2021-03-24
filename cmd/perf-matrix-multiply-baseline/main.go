package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"ulambda/perf"
)

func main() {
	e2eStart := time.Now()
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <dimension> <num-iterations>\n", os.Args[0])
		os.Exit(1)
	}

	// Get test parameters
	dim, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("Invalid dimension: %v, %v\n", os.Args[1], err)
	}
	its, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalf("Invalid num interations: %v, %v\n", os.Args[2], err)
	}

	// Create and fill matrices
	A := perf.MakeMatrix(dim, dim)
	B := perf.MakeMatrix(dim, dim)
	C := perf.MakeMatrix(dim, dim)
	A.FillRandomNonZero()
	B.FillRandomNonZero()
	C.Fill(0.0)

	compStart := time.Now()
	for i := 0; i < its; i++ {
		err = perf.Mult(A, B, C)
		if err != nil {
			log.Fatalf("Error in matrix multiply: %v", err)
		}
	}
	compEnd := time.Now()
	e2eEnd := time.Now()

	// Calculate elapsed time
	compElapsed := compEnd.Sub(compStart)
	e2eElapsed := e2eEnd.Sub(e2eStart)
	log.Printf("Total elapsed computation time: %v msec(s)\n", compElapsed.Milliseconds())
	log.Printf("Average computation time: %v msec(s)\n", compElapsed.Milliseconds()/int64(its))
	log.Printf("Total elapsed setup time: %v msec(s)\n", e2eElapsed.Milliseconds()-compElapsed.Milliseconds())
}
