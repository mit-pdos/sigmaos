package main

import (
	"log"
	"os"

	"ulambda/dbd"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: dbd <pid>")
	}
	dbd.RunDbd(os.Args[1])
}
