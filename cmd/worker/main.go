package main

import (
	"log"

	"ulambda/worker"
)

func main() {
	wr := MakeWorker()
	wr.Run()
	log.Printf("Mond: finished\n")
}
