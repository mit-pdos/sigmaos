package main

import (
	"log"
	"os"

	"ulambda/dbd"
)

func main() {
	if len(os.Args) != 1 {
		log.Fatalf("Usage: dbd")
	}
	dbd.RunDbd()
}
