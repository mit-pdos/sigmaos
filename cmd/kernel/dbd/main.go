package main

import (
	"fmt"
	"log"
	"os"

	"ulambda/dbd"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: dbd <pid>")
	}
	dbd, err := dbd.MakeDbd(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	dbd.Serve()
}
