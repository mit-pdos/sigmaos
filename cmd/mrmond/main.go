package main

import (
	"log"

	"ulambda/mr"
)

func main() {
	cr := mr.MakeMond()
	for {
		cr.Monitor()
	}
	log.Printf("Mond: finished\n")
}
