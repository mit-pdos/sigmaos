package main

import (
	"log"
	"os"

	"sigmaos/proc"
	"sigmaos/seqgrep"
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal("cannot open %s\n", os.Args[1])
	}
	seqgrep.Grep(proc.GetProcEnv(), f)
}
